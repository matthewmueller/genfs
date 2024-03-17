package genfs

type Embed struct {
	Data []byte
}

var _ FileGenerator = (*Embed)(nil)

func (e *Embed) GenerateFile(fsys FS, file *File) error {
	file.Write(e.Data)
	return nil
}
