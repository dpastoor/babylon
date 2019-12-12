package cmd

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path"
	"time"

	"os"

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
	return SGEModel{
		Nonmem: NewNonMemModel(modelname),
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
				recordConcurrentError(l.Nonmem.Model, "An error occured trying to remove the directory as specified in the overwrite flag", err, channels)
				return
			}
		} else {
			recordConcurrentError(l.Nonmem.Model, fmt.Sprintf("The target directory, %s already, exists, but we are configured to not overwrite. Invalid configuration / run state", l.Nonmem.OutputDir), errors.New("The output directory already exists"), channels)
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
		recordConcurrentError(l.Nonmem.Model, fmt.Sprintf("There appears to have been an issue trying to copy %s to %s", l.Nonmem.Model, l.Nonmem.OutputDir), err, channels)
		return
	}

	//Create Execution Script
	scriptContents, err := generateScript(scriptTemplate, l.Nonmem)

	if err != nil {
		recordConcurrentError(l.Nonmem.Model, "An error occurred during the creation of the executable script for this model", err, channels)
		return
	}

	//rwxr-x---
	afero.WriteFile(fs, path.Join(l.Nonmem.OutputDir, l.Nonmem.FileName+".sh"), scriptContents, 0750)
}

//Work describes the Turnstile execution phase -> IE What heavy lifting should be done
func (l SGEModel) Work(channels *turnstile.ChannelMap) {
	cerr := executeNonMemJob(executeSGEJob, l.Nonmem)

	if cerr.Error != nil {
		recordConcurrentError(l.Nonmem.Model, cerr.Notes, cerr.Error, channels)
	}
}

//Monitor is the 3rd phase of turnstile (not implemented here)
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

	lo := sgeOperation{}

	if verbose {
		log.Printf("setting up a work queue with %v workers", viper.GetInt("threads"))
	}

	lo.Models = sgeModelsFromArguments(args)

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

	//If we're in debug mode, let's periodically print out the details for the manager
	if debug {
		go func(m *turnstile.Manager) {
			for {
				log.Printf("Manager Details: Working: %d, Errors: %d, Completed: %d, Concurrency: %d, Iterations: %d", m.Working, m.Errors, m.Completed, m.Concurrency, m.Iterations)
				time.Sleep(500 * time.Millisecond)
			}
		}(m)
	}

	//Basically wait
	for !m.IsComplete() {
		time.Sleep(5 * time.Millisecond)
	}

	postWorkNotice(m, now)
}

func newConcurrentError(model string, notes string, err error) turnstile.ConcurrentError {
	return turnstile.ConcurrentError{
		RunIdentifier: model,
		Error:         err,
		Notes:         notes,
	}
}

func recordConcurrentError(model string, notes string, err error, channels *turnstile.ChannelMap) {
	channels.Failed <- 1
	channels.Errors <- newConcurrentError(model, notes, err)
	channels.Completed <- 1
}

func executeSGEJob(model NonMemModel) turnstile.ConcurrentError {
	log.Printf("Beginning SGE work phase for %s", model.FileName)
	fs := afero.NewOsFs()
	//Execute the script we created
	os.Chdir(model.OutputDir)
	scriptName := model.FileName + ".sh"

	//Find Qsub
	binary, err := exec.LookPath("qsub")

	if err != nil {
		newConcurrentError(model.Model, "Could not locate qsub binary in path", err)
	}

	command := exec.Command(binary, scriptName)
	command.Env = os.Environ() //Take in OS Environment

	output, err := command.CombinedOutput()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			code := exitError.ExitCode()
			details := exitError.String()

			log.Printf("Exit code was %d, details were %s", code, details)
			log.Printf("output details were: %s", string(output))
		}
		return newConcurrentError(model.Model, "Running the programmatic shell script caused an error", err)

	}

	afero.WriteFile(fs, path.Join(model.OutputDir, model.Model+".out"), output, 0750)

	return turnstile.ConcurrentError{}
}

func sgeModelsFromArguments(args []string) []SGEModel {
	var output []SGEModel
	nonmemmodels := nonmemModelsFromArguments(args)

	for _, v := range nonmemmodels {
		output = append(output, SGEModel{
			Nonmem: v,
		})
	}

	return output
}
