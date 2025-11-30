package utils

// Helper functions
func FloatPtr(f float64) *float64 {
	return &f
}

func Float64Ptr(f float64) *float64 {
	return &f
}

func BoolPtr(b bool) *bool {
	return &b
}

func StringPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}

func StringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
