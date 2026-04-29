package controllers

func boolPtr(b bool) *bool {
	return &b
}

func resolvePort(app interface{}) int {
	return 3000
}
