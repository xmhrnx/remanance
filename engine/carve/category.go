package carve

// CategoryOf is the category of a found file, which can differ from its
// signature's: a ZIP may be a Word document, RIFF a WebP picture.
func CategoryOf(f Found) Category {
	switch f.Ext {
	case "webp", "heic", "avif", "cr3":
		return Pictures
	case "avi", "wmv", "mov", "mp4", "3gp", "webm", "mkv":
		return Videos
	case "wav", "m4a", "wma", "opus", "ogg", "aiff", "rmi":
		return Audio
	case "docx", "xlsx", "pptx", "odt", "ods", "odp", "epub", "doc", "xls", "ppt", "msg", "vsd":
		return Documents
	case "apk", "jar", "ole", "dll", "exe":
		return Other
	}
	return f.Sig.Category
}

// TypeName is how a found file's type is shown in the results.
func TypeName(ext string) string {
	if n, ok := typeNames[ext]; ok {
		return n
	}
	return ext
}

var typeNames = map[string]string{
	"jpg": "JPEG", "png": "PNG", "gif": "GIF", "bmp": "Bitmap", "tif": "TIFF", "heic": "HEIC", "avif": "AVIF",
	"webp": "WebP", "psd": "Photoshop", "cr2": "Canon RAW", "cr3": "Canon RAW", "nef": "Nikon RAW",
	"arw": "Sony RAW", "dng": "DNG", "orf": "Olympus RAW", "rw2": "Panasonic RAW", "pef": "Pentax RAW", "raf": "Fujifilm RAW",
	"mp4": "MP4", "mov": "QuickTime", "3gp": "3GP", "avi": "AVI", "mkv": "Matroska", "webm": "WebM", "wmv": "Windows Media",
	"mp3": "MP3", "wav": "WAV", "m4a": "AAC", "ogg": "Ogg", "opus": "Opus", "wma": "Windows Media Audio", "aiff": "AIFF",
	"pdf": "PDF", "docx": "Word", "doc": "Word 97-2003", "xlsx": "Excel", "xls": "Excel 97-2003", "pptx": "PowerPoint",
	"ppt": "PowerPoint 97-2003", "odt": "OpenDocument Text", "ods": "OpenDocument Spreadsheet", "odp": "OpenDocument Presentation",
	"epub": "EPUB", "rtf": "Rich Text", "msg": "Outlook message",
	"zip": "ZIP", "7z": "7-Zip", "rar": "RAR", "gz": "Gzip",
	"sqlite": "SQLite", "exe": "Program", "dll": "Library", "apk": "Android app", "jar": "Java archive",
}
