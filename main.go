package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

type File struct {
	Date *time.Time
	Name string
	Path string
}

type Tasks []Task
type Task struct {
	Date     time.Time
	FilePath string
	Text     string
}

const (
	datePattern             = `^(\d{4}-\d{2}-\d{2})`
	dateHeaderPattern       = `^\#+\s+(\d{4}-\d{2}-\d{2})`
	markdownFilenamePattern = `(?i).md$`
	outputFilename          = `TASKS.md`
	taskPattern             = `^\s*[-|+|\*]?\s*\[x|\s\]`
	rootPath                = "."
	yearMonthDayLayout      = "2006-01-02"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Llongfile)

	tasks := Tasks{}
	for _, filePath := range markdownFilePaths(rootPath) {
		tasks = append(tasks, findTasks(filePath)...)
	}
	// Sort by date, keeping original order or equal elements.
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Date.Unix() < tasks[j].Date.Unix()
	})

	tasks.WriteToFile()
}

func markdownFilePaths(dirPath string) []File {
	paths := []File{}
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		date := parseDateFromFile(file)
		filename := file.Name()
		filePath := path.Join(dirPath, filename)
		if file.IsDir() {
			paths = append(paths, markdownFilePaths(filePath)...)
		} else {
			isMarkdownFile, _ := regexp.MatchString(markdownFilenamePattern, filename)
			if isMarkdownFile {
				paths = append(paths, File{Date: date, Name: file.Name(), Path: filePath})
			}
		}
	}

	return paths
}

func findTasks(file File) Tasks {
	tasks := Tasks{}
	if file.Name == outputFilename {
		return tasks
	}

	readFile, err := os.Open(file.Path)
	if err != nil {
		return tasks
	}
	defer readFile.Close()

	date := file.Date
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		line := fileScanner.Text()
		date = parseDate(dateHeaderPattern, line, date)

		if task, isTask := parseTask(*date, file.Path, line); isTask {
			tasks = append(tasks, *task)
		}
	}

	return tasks
}

func parseDate(pattern, text string, lastDate *time.Time) *time.Time {
	re := regexp.MustCompile(pattern)
	match := re.FindSubmatch([]byte(text))
	if len(match) == 2 {
		parsedDate, err := time.Parse(yearMonthDayLayout, string(match[1]))
		if err != nil {
			return lastDate
		}
		return &parsedDate
	}

	return lastDate
}

func parseDateFromFile(file fs.FileInfo) *time.Time {
	var date *time.Time
	if result := parseDate(datePattern, file.Name(), date); result != nil {
		return result
	}

	// TODO: this only works on MAC
	if call, ok := file.Sys().(*syscall.Stat_t); ok {
		result := time.Unix((*call).Birthtimespec.Sec, (*call).Birthtimespec.Nsec)
		date = &result
	}

	return date
}

func parseTask(date time.Time, filePath, line string) (*Task, bool) {
	isTask, _ := regexp.MatchString(taskPattern, line)
	if isTask {
		return &Task{
			Date:     date,
			FilePath: filePath,
			Text:     line,
		}, true

	}

	return nil, false
}

func (tasks Tasks) String() string {
	var out strings.Builder
	lastDate := time.Time{}
	for _, task := range tasks {
		// if new day, make a date header
		if task.Date.Format(yearMonthDayLayout) != lastDate.Format(yearMonthDayLayout) {
			// new line before date header if not beginning of file
			if !lastDate.IsZero() {
				out.WriteString("\n")
			}
			lastDate = task.Date
			out.WriteString(fmt.Sprintf("# %s\n\n", task.Date.Format(yearMonthDayLayout)))
		}
		out.WriteString(fmt.Sprintf("- %s\n", strings.TrimLeft(task.Text, " *-+")))
	}

	return out.String()
}

func (tasks Tasks) WriteToFile() {
	file, err := os.Create(outputFilename)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	fmt.Println("writing to file", outputFilename)
	file.WriteString(tasks.String())
}
