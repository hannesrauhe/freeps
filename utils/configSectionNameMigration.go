package utils

import (
	"fmt"
)

func getNewSectionName(name string) string {
	switch name {
	case "freepslib":
		return "fritz"
	case "openweathermap":
		return "weather"
	}
	return name
}

func migrateConfigSection(sectionsMap map[string]interface{}) (map[string]interface{}, error) {
	lowerCase := make(map[string]interface{})

	for k, v := range sectionsMap {
		lk := StringToLower(k)
		lk = getNewSectionName(lk)
		if lowerCase[lk] != nil {
			fmt.Printf("Section %s is defined in multiple case-variants in config file, preferring lower case", lk)
			if k != lk {
				continue
			}
		}
		lowerCase[lk] = v
	}

	return lowerCase, nil
}
