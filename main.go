package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"
)

type File struct {
	Path string `json:"path"`
}

// function to check if the harness can read the file or not
func IsFileBlocked(filename string) bool {
	blockedExtensions := []string{".exe", ".log", ".git", ".env"}

	// extract the extension
	ext := filepath.Ext(filename)
	// clean up by converting to lowercase
	ext = strings.ToLower(ext)

	// if the file is exactly ".env" or ".git" with no prefix, filepath.Ext might return empty. We check the base name as well.
	baseName := strings.ToLower(filepath.Base(filename))

	// compare against the blocked list
	for _, blockedExt := range blockedExtensions {
		if ext == blockedExt || baseName == blockedExt {
			return true
		}
	}

	return false // File is safe to process
}

func main() {
	// load the env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Cannot find the env file")
	}

	// get the anthropic api key
	key := os.Getenv("API_KEY")
	if key == "" {
		log.Fatal("Cannot find the anthropic api key")
	}

	// setup an anthropic client
	client := anthropic.NewClient(option.WithAPIKey(key))

	// defining tool for reading  a file
	readFileTool := anthropic.ToolParam{
		Name:        "read_file",
		Description: anthropic.String("reads the content of the file specified in the path. Use this whenever a user ask to print or read the content of the file or you need to look up whats inside a file fo anykind"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "path of file to be read"},
			},
			Required: []string{"path"},
		},
	}

	// defining tool for reading a directory to list all the files
	listFileTool := anthropic.ToolParam{
		Name:        "list_files",
		Description: anthropic.String("lists all the files in provided path directory. Use this whenever a user ask to search for a file in a specified path or list all the files in a specified directory"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "path of directory to be read"},
			},
			Required: []string{"path"},
		},
	}

	tools := []anthropic.ToolUnionParam{{OfTool: &readFileTool}, {OfTool: &listFileTool}}

	var messages []anthropic.MessageParam

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Chat session started. Type 'exit' anytime to quit.")
	fmt.Println("--------------------------------------------------")

	// print the first prompt before the loop starts
	fmt.Println("Hey Avi, how can i help you today ")
	fmt.Printf("prompt -> ")
	for scanner.Scan() {

		prompt := strings.TrimSpace(scanner.Text())

		// check for the exit command
		if strings.ToLower(prompt) == "exit" {
			fmt.Println("Bye Avi!")
			break
		}
		// ask again for an empty prompt
		if prompt == "" {

			fmt.Printf("prompt -> ")
			continue
		}
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)))

		apiCalls := 0
		for {
			apiCalls++
			fmt.Printf("[harness] API call %d \n", apiCalls)

			response, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
				Model:     anthropic.ModelClaudeOpus5_5,
				Tools:     tools,
				MaxTokens: 4096,
				Messages:  messages})
			if err != nil {
				log.Fatalf("Failed to call the anthropic API: %v", err)
			}

			// extract the text reply from the model
			var result []anthropic.ContentBlockParamUnion

			messages = append(messages, response.ToParam())

			for _, block := range response.Content {
				switch b := block.AsAny().(type) {
				case anthropic.TextBlock:
					fmt.Println(b.Text)
				case anthropic.ToolUseBlock:
					var file File
					err := json.Unmarshal([]byte(b.JSON.Input.Raw()), &file)
					if err != nil {
						result = append(result, anthropic.NewToolResultBlock(b.ID, err.Error(), true))
						log.Printf("Couldn't unmarshall the json: %v", err.Error())
						break
					}
					switch b.Name {
					case "read_file":
						if IsFileBlocked(file.Path) {
							result = append(result, anthropic.NewToolResultBlock(b.ID, "you don't have permission to read that file", true))
							log.Printf("No permission to read the file to feed the model: %v", file.Path)
							break
						}
						file_content, err := os.ReadFile(file.Path)
						if err != nil {
							result = append(result, anthropic.NewToolResultBlock(b.ID, err.Error(), true))
							log.Printf("Couldn't read the file to feed the modelL: %v", err)
							break
						}
						result = append(result, anthropic.NewToolResultBlock(b.ID, string(file_content), false))
						fmt.Printf("Tool name: %s , raw input json: %s\n", b.Name, b.JSON.Input.Raw())
						fmt.Printf("[harness] %v(%v)\n", b.Name, file.Path)
					case "list_files":
						var file_names []string
						// read contents of directory
						all_files, err := os.ReadDir(file.Path)
						if err != nil {
							result = append(result, anthropic.NewToolResultBlock(b.ID, err.Error(), true))
							log.Printf("Couldn't read the file to feed the modelL: %v\n", err)
							break
						}
						// get all the file and directory names inside specified directory in a string slice
						for _, f := range all_files {
							if f.IsDir() {
								file_names = append(file_names, f.Name()+"/")
								continue

							}
							file_names = append(file_names, f.Name())
						}
						result = append(result, anthropic.NewToolResultBlock(b.ID, strings.Join(file_names, "\n"), false))

						fmt.Printf("Tool name: %s , raw input json: %s\n", b.Name, b.JSON.Input.Raw())
						fmt.Printf("[harness] %v(%v)\n", b.Name, file.Path)
					default:
						err := fmt.Sprintf("couldn't find the tool with name: %s", b.Name)
						result = append(result, anthropic.NewToolResultBlock(b.ID, err, true))
					}

				}
			}
			if response.StopReason != anthropic.StopReasonToolUse {
				fmt.Printf("StopReason: %v\n", response.StopReason)
				break
			}
			messages = append(messages, anthropic.NewUserMessage(result...))

		}

		fmt.Println("--------------------------------------------------")
		fmt.Printf("prompt -> ")
	}
}
