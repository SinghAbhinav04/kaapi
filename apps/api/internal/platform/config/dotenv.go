package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// DotEnvFile is the local-development settings file, next to go.mod. It is git-ignored;
// .env.example at the repo root lists the names.
const DotEnvFile = ".env"

// EnvDotEnvFile picks another settings file: the dev branch runs on .env (the local
// database), and DOTENV_FILE=.env.production points a command at production (Supabase).
const EnvDotEnvFile = "DOTENV_FILE"

// DotEnvPath is the settings file to load: DOTENV_FILE when set, else .env.
func DotEnvPath() string {
	if p := os.Getenv(EnvDotEnvFile); p != "" {
		return p
	}
	return DotEnvFile
}

// LoadDotEnv copies KEY=VALUE lines from path into the environment, for local development.
// A variable that is already set wins, so real environments (Cloud Run secrets, CI) are
// never overridden. A missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, set := os.LookupEnv(key); !set && key != "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
