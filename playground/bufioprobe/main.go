package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

func main() {
	data := []byte("CAROBOOK1\n" + `{"version":1,"generator":"` + strings.Repeat("A", 2000) + `"}` + "\n")
	for _, size := range []int{16, 90, 512, 1024, 4096} {
		plain := bufio.NewReaderSize(bytes.NewReader(data), size)
		wrap := bufio.NewReaderSize(plain, 1024)
		reused := any(wrap) == any(plain)
		_, _ = wrap.ReadSlice('\n')
		meta, err := wrap.ReadSlice('\n')

		caller := bufio.NewReaderSize(bytes.NewReader(data), size)
		hidden := bufio.NewReaderSize(struct{ io.Reader }{caller}, 1024)
		hreused := any(hidden) == any(caller)
		_, _ = hidden.ReadSlice('\n')
		hmeta, herr := hidden.ReadSlice('\n')

		fmt.Printf("caller=%d plain: reused=%v metaLen=%d err=%v | hidden: reused=%v metaLen=%d err=%v\n", size, reused, len(meta), err, hreused, len(hmeta), herr)
	}
}
