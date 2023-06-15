package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

type submissionResult struct {
	Status        string `json:"status"`
	CompileOutput string `json:"compileOutput"`
	RuntimeOutput string `json:"runtimeOutput"`
}

const RUN_DIRECTORY = "/production/sandbox/"
const JAVA_DIRECTORY = "/production/jdk/bin/"
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
		var errOutput string
		if strings.Contains(err.Error(), "large") {
			errOutput = "Request body too large"
		} else {
			errOutput = "Request body is not multipart form"
		}
		http.Error(w, errOutput, http.StatusBadRequest)
		return
	}

	var codeBytes []byte
	var codeFileName string

	if codeBytes, codeFileName, err = readMultipartFile(w, r, "code", true); err != nil {
		return
	}

	if err = os.WriteFile(RUN_DIRECTORY+codeFileName, codeBytes, 0644); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var inputBytes []byte
	var inputFileName string

	if inputBytes, inputFileName, err = readMultipartFile(w, r, "input", false); err == nil {
		if err = os.WriteFile(RUN_DIRECTORY+inputFileName, inputBytes, 0644); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	var outputBytes []byte
	if outputBytes, _, err = readMultipartFile(w, r, "output", true); err != nil {
		return
	}

	outputStringList := strings.Split(string(outputBytes), "\n")
	outputStringList = formatStringList(outputStringList)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	compileOutput, compileStatus := compileCode(codeFileName)
	if !compileStatus {
		json.NewEncoder(w).Encode(submissionResult{"COMPILE_TIME_ERROR", string(compileOutput), ""})
		return
	}

	runtimeBytes, runtimeStatus := runCode(codeFileName)
	runtimeStringList := strings.Split(string(runtimeBytes), "\n")
	runtimeStringList = formatStringList(runtimeStringList)

	if runtimeStatus != "RUN_TIME_FINISHED" {
		json.NewEncoder(w).Encode(submissionResult{runtimeStatus, string(compileOutput), string(runtimeBytes)})
		return
	}

	if len(runtimeStringList) != len(outputStringList) {
		json.NewEncoder(w).Encode(submissionResult{"WRONG_ANSWER", string(compileOutput), string(runtimeBytes)})
		return
	}

	for i := 0; i < len(outputStringList); i++ {
		if outputStringList[i] != runtimeStringList[i] {
			json.NewEncoder(w).Encode(submissionResult{"WRONG_ANSWER", string(compileOutput), string(runtimeBytes)})
			return
		}
	}

	json.NewEncoder(w).Encode(submissionResult{"CORRECT_ANSWER", string(compileOutput), string(runtimeBytes)})
	return
}

func formatStringList(stringList []string) []string {
	for {
		if len(stringList) == 0 || !isWhiteSpaceOnly(stringList[0]) {
			break
		}

		stringList = stringList[1:]
	}

	for {
		if len(stringList) == 0 || !isWhiteSpaceOnly(stringList[len(stringList)-1]) {
			break
		}
		stringList = stringList[:len(stringList)-1]
	}

	for index, str := range stringList {
		stringList[index] = strings.TrimRightFunc(str, func(r rune) bool {
			return unicode.IsSpace(r)
		})
	}

	return stringList
}

func isWhiteSpaceOnly(str string) bool {
	for _, c := range str {
		if !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}

func readMultipartFile(w http.ResponseWriter, r *http.Request, name string, required bool) ([]byte, string, error) {
	file, handler, err := r.FormFile(name)
	if err != nil {
		if required {
			http.Error(w, "Requires "+name+" file", http.StatusBadRequest)
		}
		return nil, "", err
	}

	defer file.Close()
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, file); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, "", err
	}

	return buffer.Bytes(), handler.Filename, nil
}

func compileCode(codeFileName string) ([]byte, bool) {
	out, err := exec.Command(
		JAVA_DIRECTORY+"javac",
		RUN_DIRECTORY+codeFileName,
		"-Xlint:unchecked").CombinedOutput()
	return out, err == nil
}

func runCode(codeFileName string) ([]byte, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(TIMEOUT)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		JAVA_DIRECTORY+"java",
		string(codeFileName[0:strings.LastIndex(codeFileName, ".")]))

	cmd.Dir = RUN_DIRECTORY
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return out, "TIME_LIMIT_EXCEEDED"
	} else {
		if err == nil {
			return out, "RUN_TIME_FINISHED"
		} else {
			return out, "RUN_TIME_ERROR"
		}
	}
}
