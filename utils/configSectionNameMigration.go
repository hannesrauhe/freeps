package utils

func getNewSectionName(name string) string {
	switch name {
	case "freepslib":
		return "fritz"
	case "openweathermap":
		return "weather"
	}
	return name
}
