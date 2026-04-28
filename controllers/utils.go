package controllers

func boolPtr(b bool) *bool {
	return &b
}

func resolvePort(app interface{}) int {
	return 3000
}

func ptrInt32(i int32) *int32 {
	return &i
}