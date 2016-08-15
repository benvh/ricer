/*
Ricer generates configuration files based on templates.
Copyright (C) 2016  Kristof Vannotten

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

var (
	version   string
	buildDate string

	config = flag.String("c", "", "(optional) the configuration file to use")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Ricer version %s\n", version)
		fmt.Fprintf(os.Stderr, "Built %s\n", buildDate)
		fmt.Fprintln(os.Stderr, "usage:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := parseConfiguration(); err != nil {
		panic(err)
	}

	tmplDir, err := templatesDirectory()
	if err != nil {
		fmt.Printf("Templates directory does not exist, please create %s\n", tmplDir)
		return
	}

	var throttle = make(chan int, 4)
	var wg sync.WaitGroup

	files, _ := filepath.Glob(fmt.Sprintf("%s/*.tmpl", tmplDir))
	for _, file := range files {
		throttle <- 1
		wg.Add(1)

		go func(file string, wg *sync.WaitGroup, throttle chan int) {
			defer wg.Done()
			if err := handleTemplate(file); err != nil {
				fmt.Println(err)
			}
			<-throttle
		}(file, &wg, throttle)
	}

	wg.Wait()
}

func parseConfiguration() error {
	if *config == "" {
		viper.SetConfigName("config")
		configHome, err := configHomeDirectory()
		if err != nil {
			return err
		}
		viper.AddConfigPath(configHome)
	} else {
		viper.SetConfigFile(*config)
	}

	return viper.ReadInConfig()
}

func configHomeDirectory() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")

	if configHome == "" {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			usr, err := user.Current()
			if err != nil {
				return "", err
			}
			homeDir = usr.HomeDir
		}
		configHome = path.Join(homeDir, "/.config")
	}

	return path.Join(configHome, "/ricer"), nil
}

func templatesDirectory() (string, error) {
	config, err := configHomeDirectory()
	if err != nil {
		return "", err
	}

	return path.Join(config, "/templates"), nil
}

func handleTemplate(file string) error {
	var t *template.Template
	var err error

	// parse the template
	if t, err = template.ParseFiles(file); err != nil {
		return fmt.Errorf("Could not parse template %s", file)
	}

	// get the template name based on the filename
	tmplName := filepath.Base(strings.TrimSuffix(file, filepath.Ext(file)))

	// get configuration details
	m := viper.GetStringMap(fmt.Sprintf("%s.vars", tmplName))
	outputFile := viper.GetString(fmt.Sprintf("%s.output", tmplName))

	// check if an output file is given
	if outputFile == "" {
		return fmt.Errorf("You have to define an output for template %s", tmplName)
	}

	// create the path of the output file
	path := path.Dir(outputFile)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("[1] Could not create %s for template %s", outputFile, tmplName)
	}

	// write the output file
	var f *os.File
	if f, err = os.Create(outputFile); err != nil {
		return fmt.Errorf("[2] Could not create %s for template %s", outputFile, tmplName)
	}

	fmt.Printf("Creating %s from template %s.\n", outputFile, tmplName)
	t.Execute(f, m)
	f.Close()

	return nil
}
