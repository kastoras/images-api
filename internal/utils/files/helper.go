package files

import (
	"fmt"
	"mime/multipart"
)

func SafeClose(file multipart.File) {
	err := file.Close()
	if err != nil {
		fmt.Printf("problem on file closing | %v \n", err.Error())
	}
}
