// Command config prints a string from a deployed Rime config.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/rimeinn/go-rime/rime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	var options rime.Options
	flag.StringVar(&options.LibraryPath, "lib", "", "path to the librime shared library")
	flag.StringVar(&options.SharedDataDir, "shared", "", "shared Rime data directory")
	flag.StringVar(&options.UserDataDir, "user", "", "user data directory containing deployed configs")
	id := flag.String("id", "default", "deployed config ID")
	key := flag.String("key", "config_version", "config key path")
	flag.Parse()
	if options.SharedDataDir == "" || options.UserDataDir == "" {
		return fmt.Errorf("-shared and -user are required")
	}
	engine, err := rime.Open(options)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, engine.Close()) }()
	config, err := engine.OpenConfig(*id)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, config.Close()) }()
	value, ok, err := config.GetString(*key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%q does not contain a string at %q", *id, *key)
	}
	fmt.Println(value)
	return nil
}
