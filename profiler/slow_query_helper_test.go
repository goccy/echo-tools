package profiler

func ReplaceQueryDigestCommandTemplate() {
	queryDigestCommandTmpl = `echo "test %s" > %s`
}
