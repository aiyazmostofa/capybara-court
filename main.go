package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type submissionResult struct {
	Status        string `json:"status"`
	CompileOutput string `json:"compileOutput"`
	RuntimeOutput string `json:"runtimeOutput"`
}

const APP_DIRECTORY = "/home/aiyaz/Repositories/capybara-court"
const TIMEOUT = 10

func main() {
	http.HandleFunc("/", handler)
	log.Fatalln(http.ListenAndServe(":8080", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Request body needs to be POST", http.StatusMethodNotAllowed)
		return
	}

	var err error
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err = r.ParseMultipartForm(10 << 20); err != nil {
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

	if codeFileName, err = recieveFile(w, r, "code", "files/sandbox/", true); err != nil {
		log.Println(err)
		return
	}

	if _, err = recieveFile(w, r, "input", "files/sandbox/", false); err != nil {
		log.Println(err)
		return
	}

	if _, err = recieveFile(w, r, "output", "files/", true); err != nil {
		log.Println(err)
		return
	}

	compileOutput, compileStatus := compileCode(codeFileName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if !compileStatus {
		json.NewEncoder(w).Encode(submissionResult{"COMPILE_TIME_ERROR", compileOutput, ""})
		return
	}

	runtimeOutput, runtimeStatus := runCode(codeFileName)
	json.NewEncoder(w).Encode(submissionResult{runtimeStatus, compileOutput, runtimeOutput})
}

func compileCode(codeFileName string) (string, bool) {
	out, err := exec.Command(
		APP_DIRECTORY+"/jdk/bin/javac",
		APP_DIRECTORY+"/files/sandbox/"+codeFileName,
		"-Xlint:unchecked").CombinedOutput()
	return string(out), err == nil
}

func runCode(codeFileName string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(TIMEOUT)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"firejail",
		"--profile="+APP_DIRECTORY+"/firejail.cfg",
		"--quiet",
		APP_DIRECTORY+"/jdk/bin/java",
		string(codeFileName[0:strings.LastIndex(codeFileName, ".")]))

	cmd.Dir = APP_DIRECTORY + "/files/sandbox"
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(out), "TIME_LIMIT_EXCEEDED"
	} else {
		if err == nil {
			return string(out), "RUN_TIME_SUCCESS"
		} else {
			return string(out), "RUN_TIME_ERROR"
		}
	}
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
		log.Println("Unable to create file or directory")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, file); err != nil {
		log.Println("Unable to copy file")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", err
	}

	return handler.Filename, nil
}
