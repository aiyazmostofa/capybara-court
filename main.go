package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type J map[string]interface{}

var classRgx *regexp.Regexp

func writeJSON(w http.ResponseWriter, j *J) {
	resp, _ := json.Marshal(j)
	w.Header().Add("Content-Type", "application/json")
	w.Write(resp)
}

const html = `
<form method="POST" action="/" enctype="multipart/form-data">
<input type="file" name="code">
<input type="submit" value="Submit">
</form>`

func CombinedOutput(c *exec.Cmd, b *bytes.Buffer) error {
	if c.Stdout != nil {
		return errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return errors.New("exec: Stderr already set")
	}
	c.Stdout = b
	c.Stderr = b
	err := c.Run()
	return err
}

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Add("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	} else if r.Method != "POST" {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("%s is not a valid method.", r.Method)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 110*1024)

	mpf, head, err := r.FormFile("code")
	if err != nil {
		switch err.(type) {
		case *http.MaxBytesError:
			writeJSON(w, &J{"isError": "yes", "error": "Multipart form is too large."})
		default:
			writeJSON(w, &J{"isError": "yes", "error": "Could not parse multipart form."})
		}
		return
	}
	defer mpf.Close()

	className, isJava := strings.CutSuffix(head.Filename, ".java")
	if !isJava {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("%s is not a .java file.", head.Filename)})
		return
	}
	if !classRgx.MatchString(className) {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("%s is an invalid class name.", className)})
		return
	}

	dir, err := os.MkdirTemp("", "capybara-court-")
	if err != nil {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("Couldn't make a temp directory.")})
		return
	}
	defer os.RemoveAll(dir)

	file, err := os.Create(fmt.Sprintf("%s/%s.java", dir, className))
	if err != nil {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("Couldn't open the java file.")})
		return
	}
	defer file.Close()

	_, err = io.Copy(bufio.NewWriter(file), mpf)
	if err != nil {
		writeJSON(w, &J{"isError": "yes", "error": fmt.Sprintf("Couldn't copy over the java file.")})
		return
	}

	cmd := exec.Command("javac", fmt.Sprintf("%s.java", className))
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, &J{"isError": "yes",
			"error": fmt.Sprintf("COMPILE TIME ERROR!\n"+
				"-- OUTPUT --\n"+
				"%s", string(output)),
		})
		return
	}

	cmd = exec.Command("./run.sh", className, dir)
	var buffer bytes.Buffer
	err = CombinedOutput(cmd, &buffer)
	if err != nil {
		writeJSON(w, &J{"isError": "yes",
			"error": fmt.Sprintf("RUN TIME ERROR!\n"+
				"-- OUTPUT --\n"+
				"%s", string(buffer.Bytes()[:])),
		})
		return
	}

	writeJSON(w, &J{"isError": "no", "output": string(buffer.Bytes()[:])})
}

func main() {
	classRgx = regexp.MustCompile("^[A-Za-z0-9\\-_]+$")
	http.HandleFunc("/", HandleRoot)
	fmt.Println("Starting server on port 8000!")
	http.ListenAndServe(":8000", nil)
}
