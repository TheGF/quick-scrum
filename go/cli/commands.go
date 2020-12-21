package cli

import (
	"almost-scrum/core"
	"almost-scrum/web"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	log "github.com/sirupsen/logrus"
)

var config *core.Config

func usage() {
	fmt.Printf("usage: ash [-p <project-path>] <command> [<args>]\n\n" +
		"These are the common Ash commands used in various situations.\n" +
		"\tinit              Initialize a project in the project path\n" +
		"\ttop [n]           Show top stories in current store\n" +
		"\tnew [title]       Create a story\n" +
		"\tedit [name]       Edit a story\n" +
		"\tgrant [name]      Assign the story to another user\n" +
		"\tdone [name]       Change a story status to Done\n" +
		"\tcommit            Commit changes to the git repository\n" +
		"\tstore             List the stores and set the default\n" +
		"\tstore new         Create a new store\n" +
		"\tpwd <user>        Set the user's password globally\n" +
		"\tusers add <id>    Add a user to current project\n" +
		"\tusers rm <id>     Remove a user to current project\n" +
		"\tweb               Start the Web UI\n\n" +
		"",
	)
}

func processInit(projectPath string, args []string) {
	_, err := core.InitProject(projectPath)
	if err != nil {
		color.Red("Wow. Something went wrong: %v", err)
	} else {
		color.Green("Project initialized successfully in %s", projectPath)
	}
}

func setLogLevel(logLevel string) {
	logLevel = strings.ToUpper(logLevel)
	switch logLevel {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARNING":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "FATAL":
		log.SetLevel(log.FatalLevel)
	}
}

// ProcessArgs analyze the
func ProcessArgs() {
	var projectPath string
	var logLevel string
	var port string

	config = core.LoadConfig()

	flag.Usage = usage
	flag.StringVar(&projectPath, "p", ".",
		"Path where the project is. Default is current folder")

	flag.StringVar(&logLevel, "d", "error",
		"Log level to display")

	flag.StringVar(&port, "port", "8375",
		"HTTP port for the embedded web server")

	flag.Parse()

	setLogLevel(logLevel)

	commands := os.Args[1+2*flag.NFlag():]
	if len(commands) == 0 {
		flag.Usage()
		return
	}
	switch commands[0] {
	case "init":
		processInit(projectPath, commands[1:])
	case "store":
		processStore(projectPath, commands[1:])
	case "users":
		processUsers(projectPath, commands[1:])
	case "pwd":
		processPwd(commands[1:])
	case "top":
		processTop(projectPath, commands[1:])
	case "new":
		processNew(projectPath, commands[1:])
	case "edit":
		processEdit(projectPath, commands[1:])
	case "grant":
		processGrant(projectPath, commands[1:])
	// case "done":
	// 	processDone(projectPath, commands[1:])
	case "web":
		web.StartServer(port, logLevel, commands[1:])
	default:
		flag.Usage()
	}

}
