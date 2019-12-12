package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"os"

	"github.com/metrumresearchgroup/babylon/runner"
	"github.com/metrumresearchgroup/babylon/utils"
	"github.com/metrumresearchgroup/turnstile"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type sgeOperation struct {
	Models []SGEModel `json:"models"`
}

//SGEModel is the struct used for SGE operations containing the NonMemModel
type SGEModel struct {
	Nonmem NonMemModel
}

//NewSGENonMemModel create the model details from the modelname passed
func NewSGENonMemModel(modelname string) SGEModel {

	fs := afero.NewOsFs()
	lm := NonMemModel{}

	if filepath.IsAbs(modelname) {
		lm.Path = modelname
	} else {
		current, err := os.Getwd()
		if err != nil {
			lm.Error = err
		}
		lm.Path = path.Join(current, modelname)
	}

	fi, err := fs.Stat(lm.Path)

	if err != nil {
		return SGEModel{
			Nonmem: NonMemModel{
				Error: err,
			},
		}
	}

	lm.Model = fi.Name()

	modelPieces := strings.Split(lm.Model, ".")

	lm.FileName = modelPieces[0]

	//Don't assume file will have extension
	if len(modelPieces) > 1 {
		lm.Extension = modelPieces[1]
	}

	//Get the raw path of the original by stripping the actual file from it
	lm.OriginalPath = strings.Replace(lm.Path, "/"+lm.Model, "", 1)

	//If no config has been loaded, let's check to see if a config exists with the model and load it
	if viper.ConfigFileUsed() == "" {
		//Let's Set the config dir and try to load everything.
		viper.AddConfigPath(lm.OriginalPath)
		err := viper.ReadInConfig()
		if err == nil && viper.ConfigFileUsed() != "" {
			log.Printf("Config file loaded from %s%s", lm.OriginalPath, ".babylon.yml")
		}
	}

	//Process The template from the viper content for output Dir
	t, err := template.New("output").Parse(viper.GetString("outputDir"))
	buf := new(bytes.Buffer)

	if err != nil {
		return SGEModel{
			Nonmem: NonMemModel{
				Error: err,
			},
		}
	}

	type outputName struct {
		Name string
	}

	//Make sure to only use the filename for the output dir
	err = t.Execute(buf, outputName{
		Name: lm.FileName,
	})

	if err != nil {
		return SGEModel{
			Nonmem: NonMemModel{
				Error: err,
			},
		}
	}

	//Use the template content plus the original path
	lm.OutputDir = path.Join(lm.OriginalPath, buf.String())

	if err != nil {
		return SGEModel{
			Nonmem: NonMemModel{
				Error: err,
			},
		}
	}

	lm.Settings = runner.RunSettings{
		Git:                viper.GetBool("git"),
		Verbose:            verbose,
		Debug:              debug,
		CleanLvl:           viper.GetInt("cleanLvl"),
		CopyLvl:            viper.GetInt("copyLvl"),
		CacheDir:           viper.GetString("cacheDir"),
		ExeNameInCache:     viper.GetString("cacheExe"),
		NmExecutableOrPath: viper.GetString("nmExecutable"),
		OneEst:             viper.GetBool("oneEst"),
		Overwrite:          viper.GetBool("overwrite"),
	}

	return SGEModel{
		Nonmem: lm,
	}
}

//Begin Scalable method definitions

//Prepare is basically the old EstimateModel function. Responsible for creating directories and preparation.
func (l SGEModel) Prepare(channels *turnstile.ChannelMap) {
	//Mark the model as started some work
	channels.Working <- 1
	fs := afero.NewOsFs()

	//Does output directory exist?
	if ok, _ := afero.Exists(fs, l.Nonmem.OutputDir); ok {
		//If so are we configured to overwrite?
		if l.Nonmem.Settings.Overwrite {
			err := fs.RemoveAll(l.Nonmem.OutputDir)
			if err != nil {
				channels.Failed <- 1
				channels.Errors <- turnstile.ConcurrentError{
					RunIdentifier: l.Nonmem.Model,
					Error:         err,
					Notes:         "An error occured trying to remove the directory as specified in the overwrite flag",
				}
				return
			}
		} else {
			channels.Failed <- 1
			channels.Errors <- turnstile.ConcurrentError{
				RunIdentifier: l.Nonmem.Model,
				Error:         errors.New("The output directory already exists"),
				Notes:         fmt.Sprintf("The target directory, %s already, exists, but we are configured to not overwrite. Invalid configuration / run state", l.Nonmem.OutputDir),
			}
			return
		}
	}

	//Copy Model into destination and update Data Path
	err := copyFileToDestination(l.Nonmem, true)

	//Now that the directory is created, let's create the gitignore file if specified
	if viper.GetBool("git") {
		WriteGitIgnoreFile(l.Nonmem.OutputDir)
	}

	if err != nil {
		channels.Failed <- 1
		channels.Errors <- turnstile.ConcurrentError{
			RunIdentifier: l.Nonmem.Model,
			Error:         err,
			Notes:         fmt.Sprintf("There appears to have been an issue trying to copy %s to %s", l.Nonmem.Model, l.Nonmem.OutputDir),
		}
		return
	}

	//Create Execution Script
	scriptContents, err := generateScript(scriptTemplate, l.Nonmem)

	if err != nil {
		channels.Failed <- 1
		channels.Errors <- turnstile.ConcurrentError{
			RunIdentifier: l.Nonmem.Model,
			Error:         err,
			Notes:         "An error occurred during the creation of the executable script for this model",
		}
	}

	//rwxr-x---
	afero.WriteFile(fs, path.Join(l.Nonmem.OutputDir, l.Nonmem.FileName+".sh"), scriptContents, 0750)
}

//Work describes the Turnstile execution phase -> IE What heavy lifting should be done
func (l SGEModel) Work(channels *turnstile.ChannelMap) {
	log.Printf("Beginning SGE work phase for %s", l.Nonmem.FileName)
	fs := afero.NewOsFs()
	//Execute the script we created
	os.Chdir(l.Nonmem.OutputDir)
	scriptName := l.Nonmem.FileName + ".sh"

	//Find Qsub
	binary, err := exec.LookPath("qsub")

	if err != nil {
		channels.Failed <- 1
		channels.Errors <- turnstile.ConcurrentError{
			RunIdentifier: l.Nonmem.FileName,
			Error:         err,
			Notes:         "Couldn't locate qsub in path",
		}
		return
	}

	command := exec.Command(binary, scriptName)
	command.Env = os.Environ() //Take in OS Environment

	output, err := command.CombinedOutput()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			channels.Failed <- 1
			channels.Errors <- turnstile.ConcurrentError{
				RunIdentifier: l.Nonmem.FileName,
				Error:         err,
				Notes:         "Running the programmatic shell script caused an error",
			}
			code := exitError.ExitCode()
			details := exitError.String()

			log.Printf("Exit code was %d, details were %s", code, details)
			log.Printf("output details were: %s", string(output))
		}

		return
	}

	afero.WriteFile(fs, path.Join(l.Nonmem.OutputDir, l.Nonmem.Model+".out"), output, 0750)

}

//Monitor is unimplemented here. It's the 3rd phase of Turnstile execution
func (l SGEModel) Monitor(channels *turnstile.ChannelMap) {
	//Do nothing for this implementation
}

//Cleanup is the last phase of execution, in which computation / hard work is done and we're cleaning up leftover files, copying results around et all.
func (l SGEModel) Cleanup(channels *turnstile.ChannelMap) {
	log.Println("There is no cleanup phase in SGE submission. Completing task")
	channels.Completed <- 1
}

//End Scalable method definitions

// runCmd represents the run command
var sgeCMD = &cobra.Command{
	Use:   "sge",
	Short: "sge specifies to run a (set of) models on the Sun Grid Engine",
	Long:  runLongDescription,
	Run:   sge,
}

func init() {
	runCmd.AddCommand(sgeCMD)
}

func sge(cmd *cobra.Command, args []string) {

	if debug {
		viper.Debug()
	}

	AppFs := afero.NewOsFs()
	lo := sgeOperation{}

	if verbose {
		log.Printf("setting up a work queue with %v workers", viper.GetInt("threads"))
	}

	// regex for filename expansion check
	r := regexp.MustCompile("(.*)?\\[(.*)\\](.*)?")

	//Let's process our args into models
	for _, arg := range args {

		// check if arg is a file or Dir
		// dirty check for if doesn't have an extension is a folder
		_, ext := utils.FileAndExt(arg)
		if ext == "" || arg == "." {
			// could be directory, will need to be careful about the waitgroup as don't want to
			// keep waiting forever since it
			isDir, err := utils.IsDir(arg, AppFs)
			if err != nil || !isDir {
				log.Printf("issue handling %s, if this is a run please add the extension. Err: (%s)", arg, err)
				continue
			}
			modelsInDir, err := utils.ListModels(arg, ".mod", AppFs)
			if err != nil {
				log.Printf("issue getting models in dir %s, if this is a run please add the extension. Err: (%s)", arg, err)
				continue
			}
			if verbose || debug {
				log.Printf("adding %v model files in directory %s to queue", len(modelsInDir), arg)
			}

			for _, model := range modelsInDir {
				lo.Models = append(lo.Models, NewSGENonMemModel(model))
			}

		} else {
			// figure out if need to do expansion, or run as-is
			if len(r.FindAllStringSubmatch(arg, 1)) > 0 {
				log.Printf("expanding model pattern: %s \n", arg)
				pat, err := utils.ExpandNameSequence(arg)
				if err != nil {
					log.Printf("err expanding name: %v", err)
					// don't try to run this model
					continue
				}
				if verbose || debug {
					log.Printf("expanded models: %s \n", pat)
				}
				for _, p := range pat {
					lo.Models = append(lo.Models, NewSGENonMemModel(p))
				}
			} else {
				lo.Models = append(lo.Models, NewSGENonMemModel(arg))
			}
		}
	}

	if len(lo.Models) == 0 {
		log.Fatal("No models were located or loaded. Please verify the arguments provided and try again")
	}

	//Display Summary

	//Models Added
	log.Printf("A total of %d models have been located for work", len(lo.Models))

	//Models in Error
	//Locate 'em
	counter := 0
	var errors []NonMemModel
	for _, v := range lo.Models {
		if v.Nonmem.Error != nil {
			counter++
			errors = append(errors, v.Nonmem)
		}
	}

	if counter > 0 {
		log.Printf("It appears that %d models generated an error during the initial setup phase", len(errors))
		for _, v := range errors {
			log.Printf("Model named %s has errored. Details: %s", v.Model, v.Error.Error())
		}
	}

	//Models in OK state
	log.Printf("%d models successfully completed initial setup phase.", len(lo.Models)-len(errors))

	//Create signature safe slice for manager
	var scalables []turnstile.Scalable

	for _, v := range lo.Models {
		scalables = append(scalables, v)
	}

	//Begin Execution
	m := turnstile.NewManager(scalables, uint64(viper.GetInt("threads")))

	now := time.Now()

	go m.Execute()

	//Now we're doing the work. Is we done?

	//Basically wait
	for !m.IsComplete() {
		time.Sleep(5 * time.Millisecond)
	}

	fmt.Printf("\r%d models completed in %s", m.Completed, time.Since(now))
	println("")
}
