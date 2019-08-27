## bbi clean

clean files and folders

### Synopsis



glob examples:
bbi clean *.mod // anything with extension .mod
bbi clean *.mod --noFolders // anything with extension .mod
bbi clean run* // anything starting with run
regular expression examples:

bbi clean ^run --regex // anything beginning with the letters run
bbi clean ^run -v --regex // print out files and folders that will be deleted 
bbi clean ^run --filesOnly --regex // only remove matching files 
bbi clean _est_ --dirsOnly --regex // only remove matching folders  
bbi clean _est_ --dirsOnly --preview --regex // show what output would be if clean occured but don't actually clean 
bbi clean "run009.[^mod]" --regex // all matching run009.<ext> but not .mod files
bbi clean "run009.(mod|lst)$" --regex // match run009.lst and run009.mod

can also clean via the opposite of a match with inverse

bbi clean ".modt{0,1}$" --filesOnly --inverse --regex // clean all files not matching .mod or .modt

clean copied files via

bbi clean --copiedRuns="run001"
bbi clean --copiedRuns="run[001:010]"

can be a comma separated list as well

bbi clean --copiedRuns="run[001:010],run100"
 

```
bbi clean [flags]
```

### Options

```
      --copiedRuns string   run names
      --dirsOnly            only match and clean directories
      --filesOnly           only match and clean files
  -h, --help                help for clean
      --inverse             inverse selection from the given regex match criteria
      --regex               use regular expression to match instead of glob
```

### Options inherited from parent commands

```
      --config string   config file (default is $HOME/babylonconfig.toml)
  -d, --debug           debug mode
  -p, --preview         preview action, but don't actually run command
      --threads int     number of threads to execute with
  -t, --tree            json tree of output, if possible
  -v, --verbose         verbose output
```

### SEE ALSO
* [bbi](bbi.md)	 - manage and execute models

###### Auto generated by spf13/cobra on 30-Oct-2017