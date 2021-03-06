package robber

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

const (
	verbose = iota
	secret
	info
	data
	succ
	warn
	fail
)

const seperator = "--------------------------------------------------------"

var validColors = map[string]*color.Color{
	"black":     color.New(color.FgBlack),
	"blue":      color.New(color.FgBlue),
	"cyan":      color.New(color.FgCyan),
	"green":     color.New(color.FgGreen),
	"magenta":   color.New(color.FgMagenta),
	"red":       color.New(color.FgRed),
	"white":     color.New(color.FgWhite),
	"yellow":    color.New(color.FgYellow),
	"hiBlack":   color.New(color.FgHiBlack),
	"hiBlue":    color.New(color.FgHiBlue),
	"hiCyan":    color.New(color.FgHiCyan),
	"hiGreen":   color.New(color.FgHiGreen),
	"hiMagenta": color.New(color.FgHiMagenta),
	"hiRed":     color.New(color.FgHiRed),
	"hiWhite":   color.New(color.FgHiWhite),
	"hiYellow":  color.New(color.FgHiYellow),
}

// Default colors are set
var logColors = map[int]*color.Color{
	verbose: color.New(color.FgBlue),
	secret:  color.New(color.FgHiYellow).Add(color.Bold),
	info:    color.New(color.FgHiWhite),
	data:    color.New(color.FgHiBlue),
	succ:    color.New(color.FgGreen),
	warn:    color.New(color.FgRed),
	fail:    color.New(color.FgRed).Add(color.Bold),
}

type jsonFinding []struct {
	Reason        string `json:"Reason"`
	Filepath      string `json:"Filepath"`
	RepoName      string `json:"RepoName"`
	Commiter      string `json:"Commiter"`
	CommitHash    string `json:"CommitHash"`
	DateOfCommit  string `json:"DateOfCommit"`
	CommitMessage string `json:"CommitMessage"`
	Source        string `json:"Source"`
	Secret        string `json:"Secret"`
}

// Finding struct contains data of a given secret finding, used for later output of a finding.
type Finding struct {
	CommitHash    string
	CommitMessage string
	Committer     string
	DateOfCommit  string
	Email         string
	Reason        string
	Secret        []int
	Diff          string
	RepoName      string
	Filepath      string
}

// Logger handles all logging to the output.
type Logger struct {
	sync.Mutex
	Verbose bool
}

func setColors() {
	colors := GetEnvColors()
	for colorType := verbose; colorType <= fail; colorType++ {
		if empty, _ := colors[colorType]; empty == "" {
			continue
		}
		fields := strings.Fields(colors[colorType])
		if val, ok := validColors[fields[0]]; ok {
			if len(fields) > 1 && fields[1] == "bold" {
				logColors[colorType] = val.Add(color.Bold)
				continue
			}
			logColors[colorType] = val
		}
	}
}

// NewLogger sets all colors as specified and returns a new logger.
func NewLogger(verbose bool) *Logger {
	setColors()
	return &Logger{
		Verbose: verbose,
	}
}

// NewFinding simply returns a new finding struct.
func NewFinding(reason string, secret []int, diffObject *DiffObject) *Finding {
	finding := &Finding{
		CommitHash:    diffObject.Commit.Hash.String(),
		CommitMessage: diffObject.Commit.Message,
		Committer:     diffObject.Commit.Committer.Name,
		DateOfCommit:  diffObject.Commit.Committer.When.Format(time.RFC1123),
		Email:         diffObject.Commit.Committer.Email,
		Reason:        reason,
		Secret:        secret,
		RepoName:      *diffObject.Reponame,
		Filepath:      *diffObject.Filepath,
	}
	return finding
}

func saveFindingsHelper(repoName string, hash string, filePath string) string {
	if strings.HasPrefix(repoName, "/tmp") {
		return fmt.Sprintf("git --git-dir=%s show %s:%s", repoName, hash[:6], filePath)
	}
	return strings.Join([]string{repoName, "commit", hash}, "/")
}

// SaveFindings saves all findings to a JSON file named findings.json
func SaveFindings(m *Middleware) {
	var savedFindings jsonFinding
	for _, finding := range m.Findings {
		repoName := strings.Replace(finding.RepoName, ".git", "", 1)
		source := saveFindingsHelper(repoName, finding.CommitHash, finding.Filepath)
		savedFindings = append(savedFindings, jsonFinding{{
			Reason:        finding.Reason,
			Filepath:      finding.Filepath,
			RepoName:      repoName,
			Commiter:      finding.Committer,
			CommitHash:    finding.CommitHash,
			DateOfCommit:  finding.DateOfCommit,
			CommitMessage: finding.CommitMessage,
			Source:        source,
			Secret:        finding.Diff[finding.Secret[0]:finding.Secret[1]],
		}}...)
	}
	content, _ := json.MarshalIndent(savedFindings, "", "  ")
	_ = ioutil.WriteFile(*m.Flags.Save, content, 0644)
}

func (l *Logger) log(level int, format string, a ...interface{}) {
	l.Lock()
	defer l.Unlock()
	if level == verbose && l.Verbose == false {
		return
	}

	if c, ok := logColors[level]; ok {
		c.Printf(format, a...)
	} else {
		fmt.Printf(format, a...)
	}

	if level == fail {
		os.Exit(1)
	}
}

func (l *Logger) logSecret(diff string, booty []int, contextNum int) {
	data, _ := logColors[data]
	secret, _ := logColors[secret]

	data.Printf("%s", diff[:booty[0]])
	secret.Printf("%s", diff[booty[0]:booty[1]])
	data.Printf("%s\n\n", diff[booty[1]:])
}

// LogFinding is used to output Findings
func (l *Logger) LogFinding(f *Finding, m *Middleware, contextDiff string) {
	l.Lock()
	defer l.Unlock()
	f.Diff = contextDiff
	m.Append(f)

	info, _ := logColors[info]
	data, _ := logColors[data]
	secret, _ := logColors[secret]
	repoPath, _ := GetDir(f.RepoName)

	info.Println(seperator)
	info.Printf("Reason: ")
	data.Println(f.Reason)
	if f.Filepath != "" {
		info.Printf("Filepath: ")
		data.Println(f.Filepath)
	}
	info.Printf("Repo name: ")
	data.Println(strings.Replace(f.RepoName, ".git", "", 1))
	info.Printf("Committer: ")
	data.Printf("%s (%s)\n", f.Committer, f.Email)
	info.Printf("Commit hash: ")
	data.Println(f.CommitHash)
	info.Printf("View commit: ")
	data.Printf("git --git-dir=%s show %s:%s\n", repoPath, f.CommitHash[:6], f.Filepath)
	info.Printf("Date of commit: ")
	data.Println(f.DateOfCommit)
	info.Printf("Commit message: ")
	data.Printf("%s\n\n", strings.Trim(f.CommitMessage, "\n"))
	if *m.Flags.NoContext {
		secret.Printf("%s\n\n", contextDiff[f.Secret[0]:f.Secret[1]])
	} else {
		l.logSecret(f.Diff, f.Secret, *m.Flags.Context)
	}
}

// LogVerbose prints to output using 'verbose' colors
func (l *Logger) LogVerbose(format string, a ...interface{}) {
	l.log(verbose, format, a...)
}

// LogSecret prints to output using 'secret' colors
func (l *Logger) LogSecret(format string, a ...interface{}) {
	l.log(secret, format, a...)
}

// LogInfo prints to output using 'info' colors
func (l *Logger) LogInfo(format string, a ...interface{}) {
	l.log(info, "[+] "+format, a...)
}

// LogSucc prints to output using 'succ' colors
func (l *Logger) LogSucc(format string, a ...interface{}) {
	l.log(succ, "[+] "+format, a...)
}

// LogWarn prints to output using 'warn' colors
func (l *Logger) LogWarn(format string, a ...interface{}) {
	l.log(warn, "[-] "+format, a...)
}

// LogFail prints to output using 'fail' colors
func (l *Logger) LogFail(format string, a ...interface{}) {
	l.log(fail, "[!] "+format, a...)
}
