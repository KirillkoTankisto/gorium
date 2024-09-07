package main

// importing libraries /////////////////////////////
import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"slices"
	"sort"
	"sync"
	"time"
)

// variables and constants /////////////////////////
const VERSION = "0.1"
const FULL_VERSION = "Gorium " + VERSION

var backwardFabric, backwardForge = false, false
var loaderSlice = []string{"quilt", "fabric", "neoforge", "forge"}

type Config struct {
	Modsfolder  string `json:"modsfolder"`
	Gameversion string `json:"gameversion"`
	Loader      string `json:"loader"`
}

// console colors ///////////////////////////////////
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

// main function ///////////////////////////////////
func main() {
	// definition of command-line arguments
	getProject := flag.NewFlagSet("add", flag.ExitOnError)
	createProfile := flag.NewFlagSet("profile", flag.ExitOnError)
	// check if there's any arguments
	if len(os.Args) < 2 {
		fmt.Println("No arguments")
		return
	}
	// read arguments
	switch os.Args[1] {
	case "version":
		fmt.Println("Gorium", VERSION) // returns version
		return
	case "add": // adds mod to mods folder
		getProject.Parse(os.Args[2:])

		configPath, _ := getConfigPath()
		if !dirExists(configPath) {
			fmt.Println(Red, "No profile found, type gorium profile create", Reset)
			return
		}
		configdata := readConfig(configPath)

		gameversion := configdata.Gameversion
		loader := configdata.Loader
		modspath := configdata.Modsfolder

		modname := os.Args[2] // get mod's name

		latestVersion := FetchLatestVersion(modname, gameversion, loader) // get latest version's url and filename
		if latestVersion == nil {
			return
		}

		downloadFile(latestVersion.Files[0].URL, modspath, latestVersion.Files[0].Filename, nil) // downloading file
		return

	case "profile": // mod's profile
		createProfile.Parse(os.Args[2:])

		switch os.Args[2] {
		case "create": // create profile
			createConfig() // creating profile
		case "delete":
			deleteConfig()
		}
		return

	case "upgrade":
		upgrade()
		return
	case "testing":
		return
	default:
		fmt.Println("Unknown command") // error if command is incorrect
		return
	}
}

// Function for fetching latest version //////////////////////////////////////////////////////////
type File struct { // what we want to get from API
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

type Version struct { // what we check
	GameVersions  []string  `json:"game_versions"`
	VersionNumber string    `json:"version_number"`
	Loaders       []string  `json:"loaders"`
	Files         []File    `json:"files"`
	DatePublished time.Time `json:"date_published"`
}

func FetchLatestVersion(modname string, gameVersion string, loader string) *Version { // start of function

	url_project := "https://api.modrinth.com/v2/project/" + modname + "/version?game_versions=" + gameVersion // making url for GET request)

	client := http.Client{ // setting client settings for request
		Timeout: time.Second * 5,
	}

	req, _ := http.NewRequest("GET", url_project, nil) // making request

	req.Header.Set("User-Agent", FULL_VERSION) // setting identifier for program

	resp, _ := client.Do(req) // sending request

	body, _ := io.ReadAll(resp.Body) // read response

	resp.Body.Close() // closing respone

	var versions []Version
	_ = json.Unmarshal(body, &versions) // unmarshal json response

	// Enable backward compatibility
	switch loader {
	case "quilt":
		backwardFabric = true
	case "neoforge":
		backwardForge = true
	}

	// Filter and sort versions by the specified game version and loader
	var filteredVersions []Version

	for _, version := range versions {
		if !backwardForge && !backwardFabric {
			if slices.Contains(version.GameVersions, gameVersion) && slices.Contains(version.Loaders, loader) {
				filteredVersions = append(filteredVersions, version)
			}
		} else {
			if backwardForge && slices.Contains(version.GameVersions, gameVersion) && (slices.Contains(version.Loaders, loader) || slices.Contains(version.Loaders, "forge")) {
				filteredVersions = append(filteredVersions, version)
			}
			if backwardFabric && slices.Contains(version.GameVersions, gameVersion) && (slices.Contains(version.Loaders, loader) || slices.Contains(version.Loaders, "fabric")) {
				filteredVersions = append(filteredVersions, version)
			}
		}
	}
	if len(filteredVersions) == 0 {
		fmt.Println(Red, "No versions found", Reset)
		return nil // No versions found
	}
	// Sort filtered versions by DatePublished in descending order
	sort.Slice(filteredVersions, func(i, j int) bool {
		return filteredVersions[i].DatePublished.After(filteredVersions[j].DatePublished)
	})
	// Return the latest version
	return &filteredVersions[0]
}

// function to download file from url ///////////////////////////////////////////////////////////////////////////////////////////
func downloadFile(url string, modspath string, filename string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done() // Only call wg.Done() if wg is not nil
	}

	response, _ := http.Get(url)

	if !dirExists(modspath) {
		os.Mkdir(modspath, 0755)
	}

	file, _ := os.Create(path.Join(modspath, filename))
	io.Copy(file, response.Body)

	file.Close()
	response.Body.Close()
	fmt.Println(Green, "Downloaded", filename, Reset)
}

func downloadFilesConcurrently(modspath string, files []map[string]string) {
	var wg sync.WaitGroup

	for _, file := range files {
		wg.Add(1)
		go downloadFile(file["url"], modspath, file["filename"], &wg)
	}

	wg.Wait() // Wait for all downloads to finish
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func createConfig() {
	folder, mineversion, loader := getConfigDataToWrite()
	config := Config{
		Modsfolder:  path.Join(folder, ""),
		Gameversion: mineversion,
		Loader:      loader,
	}
	jsonData, _ := json.MarshalIndent(config, "", "  ")

	configpath, configfolderpath := getConfigPath()

	direxists := dirExists(configfolderpath)

	if !direxists {
		os.MkdirAll(configfolderpath, 0755)
	}

	os.WriteFile(configpath, jsonData, 0644)
}

func getConfigDataToWrite() (string, string, string) {
	var folder string
	var mineversion string
	var loader string
	for i := 0; i < 3; {
		switch i {
		case 0:
			fmt.Print("Enter mods folder path: ")
			_, _ = fmt.Scanln(&folder)
			if dirExists(folder) {
				i = 1
			}
		case 1:
			fmt.Print("Enter Minecraft version: ")
			_, _ = fmt.Scanln(&mineversion)
			if mineversion != "" {
				i = 2
			}
		case 2:
			fmt.Print("Enter loader name (quilt, fabric, neoforge, forge)\n")
			_, _ = fmt.Scanln(&loader)
			if slices.Contains(loaderSlice, loader) {
				i = 3
			}
		}
	}
	return folder, mineversion, loader
}

func readConfig(path string) Config {
	configFile, _ := os.ReadFile(path)

	var config Config
	_ = json.Unmarshal(configFile, &config)
	return config
}

func deleteConfig() {
	configpath, _ := getConfigPath()
	if !dirExists(configpath) {
		fmt.Println(Red, "No profile found to delete", Reset)
		return
	}
	os.Remove(configpath)
}

func getConfigPath() (string, string) {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".config", "gorium", "config.json"), path.Join(home, ".config", "gorium")
}

func getSHA1HashesFromDirectory(dir string) []string {
	var hashes []string

	// Reading directory contents
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := path.Join(dir, file.Name())

			hash := hashFileSHA1(filePath)

			hashes = append(hashes, hash)
		}
	}

	return hashes
}

func hashFileSHA1(filePath string) string {
	// Opening file
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Creating a new SHA1 hash
	hash := sha1.New()

	// Copying the file content to the hash
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}

	// Calculating the SHA1 checksum
	hashInBytes := hash.Sum(nil)[:20]
	hashString := hex.EncodeToString(hashInBytes)

	return hashString
}

func upgrade() {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Println(Red, "No profile found to upgrade", Reset)
		return
	}
	configdata := readConfig(configPath)

	modspath := configdata.Modsfolder
	loader := configdata.Loader
	gameversion := configdata.Gameversion

	hashes := getSHA1HashesFromDirectory(modspath)

	type HashesToSend struct {
		Hashes       []string `json:"hashes"`
		Algorithm    string   `json:"algorithm"`
		Loaders      []string `json:"loaders"`
		GameVersions []string `json:"game_versions"`
	}

	data := HashesToSend{
		Hashes:    hashes,
		Algorithm: "sha1",
		Loaders: []string{
			loader,
		},
		GameVersions: []string{
			gameversion,
		},
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")

	url := "https://api.modrinth.com/v2/version_files/update"
	url2 := "https://api.modrinth.com/v2/version_files"

	r := bytes.NewReader(jsonData)
	r2 := bytes.NewReader(jsonData)

	req, _ := http.NewRequest("POST", url, r)
	req.Header.Set("Content-Type", "application/json")  // Correct MIME type
	req.Header.Set("User-Agent", FULL_VERSION)          // Set User-Agent

	client := &http.Client{}
	resp, _ := client.Do(req) // making request

	// Repeat for the second request
	req2, _ := http.NewRequest("POST", url2, r2)
	req2.Header.Set("Content-Type", "application/json")  // Correct MIME type
	req2.Header.Set("User-Agent", FULL_VERSION)          // Set User-Agent

	resp2, _ := client.Do(req2)

	body, _ := io.ReadAll(resp.Body)
	body2, _ := io.ReadAll(resp2.Body)
	fmt.Println(body, body2)

	type File struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}

	type Root struct {
		ProjectID string `json:"project_id"`
		Files     []File `json:"files"`
	}
	// Unmarshal the JSON into a map of Root structs
	var rootMap map[string]Root
	var rootMap2 map[string]Root
	json.Unmarshal([]byte(body), &rootMap)
	json.Unmarshal([]byte(body2), &rootMap2)

	// Extract URLs and filenames and store them in a list of maps
	var fileList []map[string]string
	for _, root := range rootMap {
		for _, file := range root.Files {
			fileInfo := map[string]string{
				"url":      file.URL,
				"filename": file.Filename,
			}
			fileList = append(fileList, fileInfo)
		}
	}

	for _, root2 := range rootMap {
		for _, root3 := range rootMap2 {
			for _, file3 := range root3.Files {
				if root2.ProjectID == root3.ProjectID {
					os.Remove(path.Join(modspath, file3.Filename))
				}
			}
		}
	}

	downloadFilesConcurrently(modspath, fileList)
}
