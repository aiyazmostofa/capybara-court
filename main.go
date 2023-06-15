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

	// Recieve user code
	var codeBytes []byte
	var codeFileName string

	if codeBytes, codeFileName, err = readMultipartFile(w, r, "code", true); err != nil {
		return
	}

	if err = os.WriteFile(RUN_DIRECTORY+codeFileName, codeBytes, 0644); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Recieve sample input
	var sampleInputBytes []byte
	var sampleInputFileName string

	if sampleInputBytes, sampleInputFileName, err = readMultipartFile(w, r, "input", false); err == nil {
		if err = os.WriteFile(RUN_DIRECTORY+sampleInputFileName, sampleInputBytes, 0644); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Recieve sample output
	var sampleOutputBytes []byte
	if sampleOutputBytes, _, err = readMultipartFile(w, r, "output", true); err != nil {
		return
	}

	sampleOutputList := strings.Split(string(sampleOutputBytes), "\n")
	sampleOutputList = formatOutput(sampleOutputList)

	// Everything is now ready, add OK status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Compile code
	compileOutputBytes, compileStatus := compileCode(codeFileName)
	if !compileStatus {
		json.NewEncoder(w).Encode(submissionResult{"COMPILE_TIME_ERROR", string(compileOutputBytes), ""})
		return
	}

	// Run code
	codeOutputBytes, runtimeStatus := runCode(codeFileName)
	codeOutputList := strings.Split(string(codeOutputBytes), "\n")
	codeOutputList = formatOutput(codeOutputList)

	if runtimeStatus != "RUN_TIME_FINISHED" {
		json.NewEncoder(w).Encode(submissionResult{runtimeStatus, string(compileOutputBytes), string(codeOutputBytes)})
	} else {
		json.NewEncoder(w).Encode(submissionResult{judge(codeOutputList, sampleOutputList), string(compileOutputBytes), string(codeOutputBytes)})
	}
}

func judge(codeOutputList []string, sampleOutputList []string) string {
	if len(codeOutputList) != len(sampleOutputList) {
		return "WRONG_ANSWER"
	}

	for i := 0; i < len(codeOutputList); i++ {
		if codeOutputList[i] != sampleOutputList[i] {
			return "WRONG_ANSWER"
		}
	}
	return "CORRECT_ANSWER"
}

func formatOutput(outputList []string) []string {
	for {
		if len(outputList) == 0 || !containsOnlyWhitespace(outputList[0]) {
			break
		}

		outputList = outputList[1:]
	}

	for {
		if len(outputList) == 0 || !containsOnlyWhitespace(outputList[len(outputList)-1]) {
			break
		}
		outputList = outputList[:len(outputList)-1]
	}

	for index, line := range outputList {
		outputList[index] = strings.TrimRightFunc(line, func(r rune) bool {
			return unicode.IsSpace(r)
		})
	}

	return outputList
}

func containsOnlyWhitespace(str string) bool {
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
