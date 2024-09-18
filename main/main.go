package main

//	importing libraries
import (
	"bufio"
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
	"strconv"
	"strings"
	"sync"
	"time"

	"gorium/cli"
)

//	variables and constants
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

type File struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

type Root struct {
	ProjectID     string   `json:"project_id"`
	Files         []File   `json:"files"`
	DatePublished string   `json:"date_published"`
	Title         string   `json:"title"`
	Categories    []string `json:"categories"`
	ProjectType   string   `json:"project_type"`
	Versions      []string `json:"versions"`
}
type Searchroot struct {
	Hits []Root `json:"hits"`
}

//	console colors
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

//	main function
func main() {

	enableVirtualTerminalProcessing()

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

	getProject := flag.NewFlagSet("add", flag.ExitOnError)
	createProfile := flag.NewFlagSet("profile", flag.ExitOnError)
	searchMod := flag.NewFlagSet("search", flag.ExitOnError)

	if len(os.Args) < 2 {
		fmt.Println("Gorium Copyright © 2024 KirillkoTankisto (https://github.com/KirillkoTankisto).\nFast Minecraft CLI mod manager written in Go.\nThis program comes with ABSOLUTELY NO WARRANTY.\nThis is free software, and you are welcome to redistribute it under certain conditions.\nFor details, see here: https://www.gnu.org/licenses/gpl-3.0.txt\nContacts: kirsergeev@icloud.com, kirillkotankisto@gmail.com")
		return
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("Gorium", VERSION)
		return
	case "add":
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

		modname := os.Args[2]

		latestVersion := FetchLatestVersion(modname, gameversion, loader)
		if latestVersion == nil {
			return
		}

		downloadFile(latestVersion.Files[0].URL, modspath, latestVersion.Files[0].Filename, nil)
		return

	case "profile":
		createProfile.Parse(os.Args[2:])

		switch os.Args[2] {
		case "create":
			createConfig()
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
	case "search":
		searchMod.Parse(os.Args[2:])
		modname := os.Args[2]
		Search(modname)
	case "testing":
		return
	default:
		fmt.Println("Unknown command") // error if command is incorrect
		return
	}
}

// Function for fetching latest version //////////////////////////////////////////////////////////

type Version struct {
	GameVersions  []string  `json:"game_versions"`
	VersionNumber string    `json:"version_number"`
	Loaders       []string  `json:"loaders"`
	Files         []File    `json:"files"`
	DatePublished time.Time `json:"date_published"`
}

func FetchLatestVersion(modname string, gameVersion string, loader string) *Version {

	url_project := "https://api.modrinth.com/v2/project/" + modname + "/version?game_versions=" + gameVersion

	client := http.Client{
		Timeout: time.Second * 5,
	}

	req, _ := http.NewRequest("GET", url_project, nil)

	req.Header.Set("User-Agent", FULL_VERSION)

	resp, _ := client.Do(req)

	body, _ := io.ReadAll(resp.Body)

	resp.Body.Close()

	var versions []Version
	_ = json.Unmarshal(body, &versions)

	switch loader {
	case "quilt":
		backwardFabric = true
	case "neoforge":
		backwardForge = true
	}

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
		return nil
	}
	sort.Slice(filteredVersions, func(i, j int) bool {
		return filteredVersions[i].DatePublished.After(filteredVersions[j].DatePublished)
	})
	return &filteredVersions[0]
}

// function to download file from url ///////////////////////////////////////////////////////////////////////////////////////////
func downloadFile(url string, modspath string, filename string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	response, _ := http.Get(url)

	if !dirExists(modspath) {
		os.Mkdir(modspath, 0755)
	}

	file, _ := os.Create(path.Join(modspath, filename))
	fmt.Println("[Downloading] [" + Cyan + filename + Reset + "]")
	io.Copy(file, response.Body)

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

	for i := range oldconf.Profiles {
		oldconf.Profiles[i].Active = ""
	}

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
			fmt.Scanln(&folder)
			if dirExists(folder) {
				i = 1
			}
		case 1:
			fmt.Print("Enter Minecraft version: ")
			fmt.Scanln(&mineversion)
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
			fmt.Scanln(&name)
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
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	hash := sha1.New()

	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}

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

	if len(hashes) < 1 {
		fmt.Println("There's no mods, type gorium add")
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
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", FULL_VERSION)

	client := &http.Client{}
	resp, _ := client.Do(req)

	req2, _ := http.NewRequest("POST", url2, r2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("User-Agent", FULL_VERSION)

	resp2, _ := client.Do(req2)

	body, _ := io.ReadAll(resp.Body)
	body2, _ := io.ReadAll(resp2.Body)

	var rootMap map[string]Root
	var rootMap2 map[string]Root
	json.Unmarshal([]byte(body), &rootMap)
	json.Unmarshal([]byte(body2), &rootMap2)

	var fileList []map[string]string
	var filteredRootMap []Root
	for _, root := range rootMap {
		isDuplicate := false
		for _, root2 := range rootMap2 {
			if root.DatePublished == root2.DatePublished && root.ProjectID == root2.ProjectID {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			filteredRootMap = append(filteredRootMap, root)
		}
	}

	for _, root := range filteredRootMap {
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
				if root2.ProjectID == root3.ProjectID && root2.DatePublished != root3.DatePublished {
					os.Remove(path.Join(modspath, file3.Filename))
				}
			}
		}
	}

	if len(fileList) == 0 {
		fmt.Println("No updates found")
		return
	}

	downloadFilesConcurrently(modspath, fileList)
	fmt.Println(Green + "Upgrade completed succesfully" + Reset)
}

func switchprofile() {
	configPath, _ := getConfigPath()
	roots := readFullConfig(configPath)
	switchmenu := cli.NewMenu("Choose profile")
	for _, profile := range roots.Profiles {
		if profile.Active == "*" {
			switchmenu.AddItem(profile.Name+Reset+" ["+Green+"Active"+Reset+"] ["+Cyan+profile.Loader+Reset+", "+Yellow+profile.Gameversion+Reset+"] ["+White+profile.Modsfolder+Reset+"]", profile.Name)
		} else {
			switchmenu.AddItem(profile.Name+Reset+" ["+Cyan+profile.Loader+Reset+", "+Yellow+profile.Gameversion+Reset+"] ["+White+profile.Modsfolder+Reset+"]", profile.Name)
		}
	}
	if len(switchmenu.MenuItems) < 2 {
		fmt.Println("No profiles to switch")
		return
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
	for _, profile := range roots.Profiles {
		if profile.Active == "*" {
			fmt.Println(profile.Name + Reset + " [" + Green + "Active" + Reset + "] [" + Cyan + profile.Loader + Reset + ", " + Yellow + profile.Gameversion + Reset + "] [" + White + profile.Modsfolder + Reset + "]")
		} else {
			fmt.Println(profile.Name + Reset + " [" + Cyan + profile.Loader + Reset + ", " + Yellow + profile.Gameversion + Reset + "] [" + White + profile.Modsfolder + Reset + "]")
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

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func Search(modname string) {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
		return
	}
	configdata := readConfig(configPath)

	loader := configdata.Loader
	version := configdata.Gameversion
	modspath := configdata.Modsfolder

	switch loader {
	case "quilt":
		backwardFabric = true
	case "neoforge":
		backwardForge = true
	}

	url_project := fmt.Sprintf("https://api.modrinth.com/v2/search?query=%s&limit=100", modname)
	client := http.Client{
		Timeout: time.Second * 5,
	}

	req, _ := http.NewRequest("GET", url_project, nil)
	req.Header.Set("User-Agent", FULL_VERSION)

	resp, _ := client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var results Searchroot
	_ = json.Unmarshal(body, &results)

	var sortedresults Searchroot

	for _, hit := range results.Hits {
		if hit.ProjectType == "mod" && contains(hit.Versions, version) {
			if backwardForge {
				if contains(hit.Categories, loader) || contains(hit.Categories, "forge") {
					sortedresults.Hits = append(sortedresults.Hits, hit)
				}
			} else if backwardFabric {
				if contains(hit.Categories, loader) || contains(hit.Categories, "fabric") {
					sortedresults.Hits = append(sortedresults.Hits, hit)
				}
			} else {
				if contains(hit.Categories, loader) {
					sortedresults.Hits = append(sortedresults.Hits, hit)
				}
			}
		}
	}

	if len(sortedresults.Hits) == 0 {
		fmt.Println(Red + "No results found" + Reset)
		return
	}

	for i := len(sortedresults.Hits) - 1; i >= 0; i-- {
		fmt.Printf("[%d] %s\n", i+1, sortedresults.Hits[i].Title)
	}

	in := bufio.NewReader(os.Stdin)

	fmt.Print("Enter numbers of mods you want to install: ")

	selected, _ := in.ReadString('\n')

	selected = strings.TrimSpace(selected)

	substrings := strings.Split(selected, " ")

	var selected_integers []int
	for _, rand := range substrings {
		str, _ := strconv.Atoi(rand)
		selected_integers = append(selected_integers, str-1)
	}

	var mods_to_download []string
	for i := range selected_integers {
		mods_to_download = append(mods_to_download, sortedresults.Hits[selected_integers[i]].ProjectID)
	}

	type Versions struct {
		Version []*Version
	}
	var latestVersions Versions
	for i := range mods_to_download {
		latestVersions.Version = append(latestVersions.Version, FetchLatestVersion(mods_to_download[i], version, loader))
	}

	var files_to_download []map[string]string
	for _, root := range latestVersions.Version {
		for _, file := range root.Files {
			fileInfo := map[string]string{
				"url":      file.URL,
				"filename": file.Filename,
			}
			if !strings.Contains(fileInfo["filename"], "sources") {
				files_to_download = append(files_to_download, fileInfo)
			}
		}
	}
	downloadFilesConcurrently(modspath, files_to_download)
}
