package configlite

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Values map[string]string

func Load(path string) (Values, error) {
	if path == "" {
		return Values{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Values{}, nil
		}
		return nil, err
	}
	defer file.Close()

	values := Values{}
	var section string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := normalizeValue(parts[1])
		if indent > 0 && section != "" {
			key = section + "." + key
		}
		values[key] = value
	}
	return values, scanner.Err()
}

func PathFromArgs(args []string, flagName string) string {
	name := "-" + flagName
	longName := "--" + flagName
	for i, arg := range args {
		if arg == name || arg == longName {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"=")
		}
		if strings.HasPrefix(arg, longName+"=") {
			return strings.TrimPrefix(arg, longName+"=")
		}
	}
	return ""
}

func (v Values) String(key, fallback string) string {
	if value := v[key]; value != "" {
		return value
	}
	return fallback
}

func (v Values) Int(key string, fallback int) int {
	value := v[key]
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (v Values) Bool(key string, fallback bool) bool {
	value := v[key]
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func normalizeValue(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "\"'")
	return value
}
