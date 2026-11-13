// Command console demonstrates input, candidate selection, and commits.
package main

import (
	"bufio"
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
	flag.StringVar(&options.SharedDataDir, "shared", "", "directory containing Rime schemas and dictionaries")
	flag.StringVar(&options.UserDataDir, "user", "", "writable Rime user data directory")
	schema := flag.String("schema", "", "schema ID to select")
	deploy := flag.Bool("deploy", false, "deploy the workspace before creating a session")
	flag.Parse()
	if options.SharedDataDir == "" || options.UserDataDir == "" {
		return fmt.Errorf("-shared and -user are required")
	}
	engine, err := rime.Open(options)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, engine.Close()) }()
	if *deploy {
		if err := engine.Deploy(); err != nil {
			return err
		}
	}
	session, err := engine.NewSession()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, session.Close()) }()
	if *schema != "" {
		selected, err := session.SelectSchema(*schema)
		if err != nil {
			return err
		}
		if !selected {
			return fmt.Errorf("could not select schema %q", *schema)
		}
	}

	fmt.Fprintln(os.Stderr, "Enter ASCII input, one line at a time. The first candidate is committed if present.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if err := inputLine(session, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func inputLine(session *rime.Session, line string) error {
	for _, key := range line {
		if key > 127 {
			return fmt.Errorf("non-ASCII key %q", key)
		}
		if _, err := session.ProcessKey(key, 0); err != nil {
			return err
		}
	}
	context, ok, err := session.Context()
	if err != nil {
		return err
	}
	if ok && len(context.Menu.Candidates) > 0 {
		for i, candidate := range context.Menu.Candidates {
			fmt.Printf("%d. %s %s\n", i+1, candidate.Text, candidate.Comment)
		}
		if _, err := session.SelectCandidateOnCurrentPage(0); err != nil {
			return err
		}
	} else if _, err := session.CommitComposition(); err != nil {
		return err
	}
	text, ok, err := session.TakeCommit()
	if ok {
		fmt.Println(text)
	}
	return err
}
