package text

import (
	"fmt"
	"strings"
)

func Coalesce(first string, other ...string) string {
	fmt.Println("######## string_Coalesce")

	res := first
	for i := range other {
		if res != "" {
			break
		}
		res = other[i]
	}
	return res
}

func SplitRemoveEmpty(in string, separator string) []string {
	fmt.Println("######## string_SplitRemoveEmpty")

	var res []string
	for _, item := range strings.Split(in, separator) {
		if item != "" {
			res = append(res, item)
		}
	}
	return res
}
