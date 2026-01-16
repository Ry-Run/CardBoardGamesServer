package utils

func Contains(data []string, include string) bool {
	for _, v := range data {
		if v == include {
			return true
		}
	}
	return false
}
