package utils

import (
	"crypto"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetValueForETagUsingAttributes(timeIn time.Time, sizeIn int64) string {
	theHash := crypto.SHA1.New()
	theValue := timeIn.Format(http.TimeFormat) + ":" + strconv.FormatInt(sizeIn, 10)
	theSum := theHash.Sum([]byte(theValue))
	theHash.Reset()
	if len(theSum) > 0 {
		return "\"" + hex.EncodeToString(theSum) + "\""
	} else {
		return "\"" + hex.EncodeToString([]byte(theValue)) + "\""
	}
}

func GetETagValues(stringIn string) []string {
	if strings.ContainsAny(stringIn, ",") {
		seperated := strings.Split(stringIn, ",")
		toReturn := make([]string, len(seperated))
		pos := 0
		for _, s := range seperated {
			cETag := GetETagValue(s)
			if cETag != "" {
				toReturn[pos] = cETag
				pos += 1
			}
		}
		if pos == 0 {
			return nil
		}
		return toReturn[:pos]
	}
	toReturn := []string{GetETagValue(stringIn)}
	if toReturn[0] == "" {
		return nil
	}
	return toReturn
}

func GetETagValue(stringIn string) string {
	startIndex := strings.IndexAny(stringIn, "\"") + 1
	endIndex := strings.LastIndexAny(stringIn, "\"")
	if endIndex > startIndex {
		return stringIn[startIndex:endIndex]
	}
	return ""
}
