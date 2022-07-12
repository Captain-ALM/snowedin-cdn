package utils

import (
	"mime"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/conf"
	"strconv"
	"strings"
	"time"
)

func LengthOfStringSlice(theSlice []string) int {
	theLength := 0
	for _, i := range theSlice {
		theLength += len(i)
	}
	theLength += (len(theSlice) - 1) * 2
	return theLength
}

func SetNeverCacheHeader(header http.Header) {
	header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
	header.Set("Pragma", "no-cache")
}

func SetExpiresHeader(header http.Header, expireTime time.Time) {
	header.Set("Expires", expireTime.UTC().Format(http.TimeFormat))
}

func SetLastModifiedHeader(header http.Header, modTime time.Time) {
	if !modTime.IsZero() {
		header.Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	}
}

func SetCacheHeaderWithAge(header http.Header, maxAge uint, modifiedTime time.Time, isPrivate bool) {
	header.Set("Cache-Control", "max-age="+strconv.Itoa(int(maxAge))+", must-revalidate")
	if isPrivate {
		header.Set("Cache-Control", header.Get("Cache-Control")+", private")
	}
	if maxAge > 0 {
		checkerSecondsBetween := int64(time.Now().UTC().Sub(modifiedTime.UTC()).Seconds())
		if checkerSecondsBetween < 0 {
			checkerSecondsBetween *= -1
		}
		header.Set("Age", strconv.FormatUint(uint64(checkerSecondsBetween)%uint64(maxAge), 10))
	}
}

func SwitchToNonCachingHeaders(header http.Header) {
	SetNeverCacheHeader(header)
	if header.Get("Last-Modified") != "" {
		header.Del("Last-Modified")
	}
	if header.Get("Age") != "" {
		header.Del("Age")
	}
	if header.Get("Expires") != "" {
		header.Del("Expires")
	}
	if header.Get("ETag") != "" {
		header.Del("ETag")
	}
}

func GetFilenameFromPath(pathIn string) string {
	lastSlashIndex := strings.LastIndexAny(pathIn, "/")
	if lastSlashIndex < 0 {
		return pathIn
	} else {
		return pathIn[lastSlashIndex+1:]
	}
}

func SetDownloadHeaders(header http.Header, config conf.DownloadSettingsYaml, filename string, mimeType string) {
	if config.OutputFilename {
		theFilename := filename
		if theFilename == "" {
			theFilename = "download"
		}
		if exts, err := mime.ExtensionsByType(mimeType); config.SetExtensionIfMissing && !strings.Contains(theFilename, ".") && err == nil && len(exts) > 0 {
			theFilename += exts[0]
		}
		header.Set("Content-Disposition", "attachment; filename=\""+theFilename+"\"")
	} else {
		header.Set("Content-Disposition", "attachment")
	}
}
