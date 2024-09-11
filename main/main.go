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
	"strings"
	"sync"
	"time"

	"gorium/cli"

	"github.com/schollz/progressbar/v3"
)

// variables and constants /////////////////////////
const VERSION = "0.1"
const FULL_VERSION = "Gorium " + VERSION

var backwardFabric, backwardForge = false, false
var loaderSlice = []string{"quilt", "fabric", "neoforge", "forge"}

type Config struct {
	Active      string `json:"active"`
	Name        string `json:"name"`
	Modsfolder  string `json:"modsfolder"`
	Gameversion string `json:"gameversion"`
	Loader      string `json:"loader"`
}

type MultiConfig struct {
	Profiles []Config
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

	configpath, configfolder := getConfigPath()

	if !dirExists(configpath) {
		datatowrite := MultiConfig{
			Profiles: []Config{},
		}

		jsonData, _ := json.MarshalIndent(datatowrite, "", "  ")

		if !dirExists(configfolder) {
			os.MkdirAll(configfolder, 0755)
		}

		os.WriteFile(configpath, jsonData, 0644)
	}

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
			fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
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
		case "switch":
			switchprofile()
		case "list":
			listprofiles()
			return
		}
		return

	case "upgrade":
		upgrade()
		return
	case "list":
		listmods()
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
		fmt.Println(Red + "No versions found" + Reset)
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
	bar := progressbar.DefaultBytes(response.ContentLength, "Downloading "+Cyan+filename+Reset)
	io.Copy(io.MultiWriter(file, bar), response.Body)

	file.Close()
	response.Body.Close()
}

func downloadFilesConcurrently(modspath string, urls []map[string]string) {
	var wg sync.WaitGroup
	wg.Add(len(urls))

	for _, urlMap := range urls {
		go downloadFile(urlMap["url"], modspath, urlMap["filename"], &wg)
	}

	wg.Wait()
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
	folder, mineversion, loader, name := getConfigDataToWrite()

	newConfig := Config{
		Modsfolder:  path.Join(folder, ""),
		Gameversion: mineversion,
		Loader:      loader,
		Name:        name,
		Active:      "*",
	}

	configpath, _ := getConfigPath()

	oldconf := readFullConfig(configpath)

	oldconf.Profiles = append(oldconf.Profiles, newConfig)

	jsonData, _ := json.MarshalIndent(oldconf, "", "  ")

	os.WriteFile(configpath, jsonData, 0644)
}

func getConfigDataToWrite() (string, string, string, string) {
	var folder string
	var mineversion string
	var loader string
	var name string
	for i := 0; i < 4; {
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
			menu := cli.NewMenu("Choose loader")
			menu.AddItem("Quilt", "quilt")
			menu.AddItem("Fabric", "fabric")
			menu.AddItem("Forge", "forge")
			menu.AddItem("Neoforge", "neoforge")
			loader = menu.Display()
			i = 3
		case 3:
			fmt.Print("How does this profile should be called?\n")
			_, _ = fmt.Scanln(&name)
			if name != "" {
				i = 4
			}
		}
	}
	return folder, mineversion, loader, name
}

func readConfig(path string) Config {
	configFile, _ := os.ReadFile(path)

	var config MultiConfig
	json.Unmarshal(configFile, &config)
	for _, rootconfig := range config.Profiles {
		if rootconfig.Active == "*" {
			return rootconfig
		}
	}
	return Config{}
}

func readFullConfig(path string) MultiConfig {
	configFile, _ := os.ReadFile(path)

	var config MultiConfig
	json.Unmarshal(configFile, &config)
	return config
}

func deleteConfig() {
	configpath, _ := getConfigPath()
	if !dirExists(configpath) {
		fmt.Println(Red + "No profile found to delete" + Reset)
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
		fmt.Println(Red + "No profile found to upgrade" + Reset)
		return
	}
	configdata := readConfig(configPath)

	modspath := configdata.Modsfolder
	loader := configdata.Loader
	gameversion := configdata.Gameversion

	hashes := getSHA1HashesFromDirectory(modspath)

	if len(hashes) == 0 {
		return
	}

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
	req.Header.Set("Content-Type", "application/json") // Correct MIME type
	req.Header.Set("User-Agent", FULL_VERSION)         // Set User-Agent

	client := &http.Client{}
	resp, _ := client.Do(req) // making request

	// Repeat for the second request
	req2, _ := http.NewRequest("POST", url2, r2)
	req2.Header.Set("Content-Type", "application/json") // Correct MIME type
	req2.Header.Set("User-Agent", FULL_VERSION)         // Set User-Agent

	resp2, _ := client.Do(req2)

	body, _ := io.ReadAll(resp.Body)
	body2, _ := io.ReadAll(resp2.Body)

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
			if !strings.Contains(fileInfo["filename"], "sources") {
				fileList = append(fileList, fileInfo)
			}
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
	fmt.Println(Green + "Upgrade completed succesfully" + Reset)
}

func switchprofile() {
	configPath, _ := getConfigPath()
	roots := readFullConfig(configPath)
	switchmenu := cli.NewMenu("Choose profile")
	for _, profile := range roots.Profiles {
		switchmenu.AddItem(profile.Name+" "+profile.Loader+" "+profile.Gameversion, profile.Name)
	}
	choosenprofile := switchmenu.Display()
	for i := range roots.Profiles {
		if roots.Profiles[i].Active == "*" {
			roots.Profiles[i].Active = ""
		}
		if roots.Profiles[i].Name == choosenprofile {
			roots.Profiles[i].Active = "*"
		}
	}

	jsonData, _ := json.MarshalIndent(roots, "", "  ")

	os.WriteFile(configPath, jsonData, 0644)
}

func listprofiles() {
	configPath, _ := getConfigPath()
	roots := readFullConfig(configPath)
	for _, root := range roots.Profiles {
		if root.Active == "*" {
			fmt.Println(root.Active, Cyan+root.Name+Reset)
		} else {
			fmt.Println(root.Name)
		}

	}
}

func listmods() {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
		return
	}
	configdata := readConfig(configPath)
	modsfolder := configdata.Modsfolder
	mods, _ := os.ReadDir(modsfolder)
	if len(mods) == 0 {
		fmt.Println("No mods, type gorium add")
		return
	}
	for _, mod := range mods {
		fmt.Println(Cyan + mod.Name() + Reset)
	}
}
