package utils

import (
	"strconv"
	"strings"
)

type ContentRangeValue struct {
	Start, Length int64
}

func (rstrc ContentRangeValue) ToField(maxLength int64) string {
	return "bytes " + strconv.FormatInt(rstrc.Start, 10) + "-" + strconv.FormatInt(rstrc.Start+rstrc.Length-1, 10) + "/" + strconv.FormatInt(maxLength, 10)
}

func GetRanges(rangeStringIn string, maxLength int64) []ContentRangeValue {
	actualRangeString := strings.TrimPrefix(rangeStringIn, "bytes=")
	if strings.ContainsAny(actualRangeString, ",") {
		seperated := strings.Split(actualRangeString, ",")
		toReturn := make([]ContentRangeValue, len(seperated))
		pos := 0
		for _, s := range seperated {
			if cRange, ok := GetRange(s, maxLength); ok {
				toReturn[pos] = cRange
				pos += 1
			}
		}
		if pos == 0 {
			return nil
		}
		return toReturn[:pos]
	}
	if cRange, ok := GetRange(actualRangeString, maxLength); ok {
		return []ContentRangeValue{cRange}
	}
	return nil
}

func GetRange(rangePartIn string, maxLength int64) (ContentRangeValue, bool) {
	before, after, done := strings.Cut(rangePartIn, "-")
	before = strings.Trim(before, " ")
	after = strings.Trim(after, " ")
	if !done {
		return ContentRangeValue{}, false
	}
	var parsedAfter, parsedBefore int64 = -1, -1
	if after != "" {
		if parsed, err := strconv.ParseInt(after, 10, 64); err == nil {
			parsedAfter = parsed
		} else {
			return ContentRangeValue{}, false
		}
	}
	if before != "" {
		if parsed, err := strconv.ParseInt(before, 10, 64); err == nil {
			parsedBefore = parsed
		} else {
			return ContentRangeValue{}, false
		}
	}
	if parsedBefore >= 0 && parsedAfter > parsedBefore && parsedAfter < maxLength {
		return ContentRangeValue{
			Start:  parsedBefore,
			Length: parsedAfter - parsedBefore + 1,
		}, true
	} else if parsedAfter < 0 && parsedBefore >= 0 && parsedBefore < maxLength {
		return ContentRangeValue{
			Start:  parsedBefore,
			Length: maxLength - parsedBefore,
		}, true
	} else if parsedBefore < 0 && parsedAfter >= 1 && maxLength-parsedAfter >= 0 {
		return ContentRangeValue{
			Start:  maxLength - parsedAfter,
			Length: parsedAfter,
		}, true
	}
	return ContentRangeValue{}, false
}
