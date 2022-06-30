package cdn

import (
	"net/http"
	"time"
)

func NewZonePathAttributes(lModTime time.Time, eTag string) *ZonePathAttributes {
	return &ZonePathAttributes{
		lastModifiedTime: lModTime,
		eTag:             eTag,
		NotExpunged:      true,
	}
}

type ZonePathAttributes struct {
	lastModifiedTime time.Time
	eTag             string
	contentLength    string
	contentType      string
	cacheControl     string
	age              string
	expire           string
	NotExpunged      bool
}

func (zpa *ZonePathAttributes) Update(lModTime time.Time, eTag string, header http.Header) {
	zpa.NotExpunged = true
	zpa.lastModifiedTime = lModTime
	zpa.eTag = eTag
	zpa.contentLength = header.Get("Content-Length")
	zpa.contentType = header.Get("Content-Type")
	zpa.cacheControl = header.Get("Cache-Control")
	zpa.age = header.Get("Age")
	zpa.expire = header.Get("Expires")
}

func (zpa *ZonePathAttributes) UpdateHeader(header http.Header) {
	if zpa.NotExpunged {
		if zpa.contentLength != "" {
			header.Set("Content-Length", zpa.contentLength)
		}
		if zpa.contentType != "" {
			header.Set("Content-Type", zpa.contentType)
		}
		if zpa.cacheControl != "" {
			header.Set("Cache-Control", zpa.cacheControl)
		}
		if zpa.age != "" {
			header.Set("Age", zpa.age)
		}
		if zpa.expire != "" {
			header.Set("Expires", zpa.expire)
		}
	}
}
