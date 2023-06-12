package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func main() {
	http.HandleFunc("/", postHandler)
	log.Fatalln(http.ListenAndServe(":8080", nil))
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err = r.ParseMultipartForm(5 << 20); err != nil {
		log.Println(err)
		var errOutput string
		if strings.Contains(err.Error(), "large") {
			errOutput = "Request body too large"
		} else {
			errOutput = "Request body is not multipart form"
		}
		http.Error(w, errOutput, http.StatusBadRequest)
		return
	}

	var codeFileName string
	var outputFileName string

	if codeFileName, err = recieveFile(w, r, "code", "files/sandbox/", true); err != nil {
		log.Println(err)
		return
	}

	if _, err = recieveFile(w, r, "input", "files/sandbox/", false); err != nil {
		log.Println(err)
		return
	}

	if outputFileName, err = recieveFile(w, r, "output", "files/", true); err != nil {
		log.Println(err)
		return
	}

	compileOutput, _ := compileCode(codeFileName, outputFileName)
	w.Write(([]byte)(compileOutput))
}

func compileCode(codeFileName string, outputFileName string) (string, bool) {
	out, err := exec.Command("javac", "files/sandbox/"+codeFileName).CombinedOutput()
	return string(out), err == nil
}

func recieveFile(w http.ResponseWriter, r *http.Request, name string, directory string, required bool) (string, error) {
	file, handler, err := r.FormFile(name)
	if err != nil {
		if !required {
			return "", nil
		}

		http.Error(w, "Expected "+name+" file", http.StatusBadRequest)
		return "", err
	}
	defer file.Close()

	destination, err := os.Create(directory + handler.Filename)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, file); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", err
	}

	return handler.Filename, nil
}
