package ports

type Clipboard interface {
	Copy(text string) error
}

type Editor interface {
	Open(filePath string) error
}

type Tracer interface {
	Enabled() bool
	Event(event string, fields map[string]any)
}
