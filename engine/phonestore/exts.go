package phonestore

import "strings"

// skip are folders not worth walking: they hold app packages. Apps'
// own folders, Android/data, are walked: Android denies them to other
// apps, so on the phone the walk passes them by, but over ADB they can
// be read, and gallery and chat apps keep thumbnails and copies of
// photos there that outlive the originals.
var skip = map[string]bool{"Android/obb": true}

// cacheDir names folders apps keep caches in.
var cacheDir = map[string]bool{"cache": true, "caches": true, "thumbnails": true, "thumbs": true}

// media are the extensions of photos, videos and recordings.
var media = map[string]bool{}

func init() {
	for _, e := range strings.Fields("jpg jpeg png gif webp heic heif bmp dng raw " +
		"mp4 m4v mov 3gp 3g2 mkv webm avi " +
		"mp3 m4a aac amr ogg opus wav flac") {
		media[e] = true
	}
}
