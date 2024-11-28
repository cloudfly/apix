package apix

var (
	DefaultService *Service
)

func init() {
	DefaultService = New()
}

func ListenAndServe(addr string) error { return DefaultService.ListenAndServe(addr) }

func ANY(path string, h any)     { DefaultService.ANY(path, h) }
func GET(path string, h any)     { DefaultService.GET(path, h) }
func POST(path string, h any)    { DefaultService.POST(path, h) }
func PUT(path string, h any)     { DefaultService.PUT(path, h) }
func PATCH(path string, h any)   { DefaultService.PATCH(path, h) }
func DELETE(path string, h any)  { DefaultService.DELETE(path, h) }
func TRACE(path string, h any)   { DefaultService.TRACE(path, h) }
func HEAD(path string, h any)    { DefaultService.HEAD(path, h) }
func OPTION(path string, h any)  { DefaultService.OPTION(path, h) }
func CONNECT(path string, h any) { DefaultService.CONNECT(path, h) }
