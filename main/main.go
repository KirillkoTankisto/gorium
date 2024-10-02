package main

//	importing libraries
import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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

	"golang.org/x/term"
)

// ProgramVersion variables and constants
const ProgramVersion = "0.1"
const FullVersion = "Gorium " + ProgramVersion

var helpStrings = []string{
	"Use: gorium <command>",
	"",
	"gorium add <mod slug/id> - add mod",
	"gorium help - display this text",
	"gorium list - list installed mods",
	"gorium profile <create/delete/switch/list>",
	"gorium search - search mods through Modrinth",
	"gorium upgrade - update mods to latest version",
	"gorium version - display current version of Gorium",
}

var licenseStrings = []string{
	"Gorium Copyright © 2024 KirillkoTankisto (https://github.com/KirillkoTankisto).",
	"",
	"Fast Minecraft CLI mod manager written in Go.",
	"This program comes with ABSOLUTELY NO WARRANTY.",
	"This is free software, and you are welcome to redistribute it under certain conditions.",
	"For details, see here: https://www.gnu.org/licenses/gpl-3.0.txt",
	"Contacts: kirsergeev@icloud.com, kirillkotankisto@gmail.com",
}

type Config struct {
	Active      string `json:"active"`
	Name        string `json:"name"`
	ModsFolder  string `json:"modsfolder"`
	GameVersion string `json:"gameversion"`
	Loader      string `json:"loader"`
	Hash        string `json:"hash"`
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
	Name          string   `json:"name"`
}
type SearchRoot struct {
	Hits []Root `json:"hits"`
}

type HashesToSend struct {
	Hashes       []string `json:"hashes"`
	Algorithm    string   `json:"algorithm"`
	Loaders      []string `json:"loaders"`
	GameVersions []string `json:"game_versions"`
}

type Version struct {
	GameVersions  []string  `json:"game_versions"`
	VersionNumber string    `json:"version_number"`
	Loaders       []string  `json:"loaders"`
	Files         []File    `json:"files"`
	DatePublished time.Time `json:"date_published"`
}

// console colors and format
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
	Bold   = "\033[1m"
	Italic = "\033[3m"
)

// main function
func main() {

	enableVirtualTerminalProcessing()

	configPath, configFolder := getConfigPath()

	if !dirExists(configPath) {
		dataToWrite := MultiConfig{
			Profiles: []Config{},
		}

		jsonData, _ := json.MarshalIndent(dataToWrite, "", "  ")

		if !dirExists(configFolder) {
			err := os.MkdirAll(configFolder, 0755)
			checkError(err)
		}

		err := os.WriteFile(configPath, jsonData, 0644)
		checkError(err)
	}

	configData := readConfig(configPath)

	loader := configData.Loader
	backwardFabric, backwardForge := false, false
	switch loader {
	case "quilt":
		backwardFabric = true
	case "neoforge":
		backwardForge = true
	}
	backward := []bool{backwardFabric, backwardForge} // 0 = Fabric, 1 = Forge

	getProject := flag.NewFlagSet("add", flag.ExitOnError)
	createProfile := flag.NewFlagSet("profile", flag.ExitOnError)
	searchMod := flag.NewFlagSet("search", flag.ExitOnError)

	if len(os.Args) < 2 {
		displaySimpleText(licenseStrings)
		return
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("Gorium", ProgramVersion)
		return
	case "add":
		err := getProject.Parse(os.Args[2:])
		checkError(err)

		configPath, _ := getConfigPath()
		if !dirExists(configPath) {
			fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
			return
		}
		configData := readConfig(configPath)

		gameVersion := configData.GameVersion
		loader := configData.Loader
		modsPath := configData.ModsFolder

		modName := os.Args[2]

		latestVersion := fetchLatestVersion(modName, gameVersion, loader, backward)
		if latestVersion == nil {
			return
		}

		err = downloadFile(latestVersion.Files[0].URL, modsPath, latestVersion.Files[0].Filename)
		checkError(err)
		return

	case "profile":
		err := createProfile.Parse(os.Args[2:])
		checkError(err)

		switch os.Args[2] {
		case "create":
			createConfig()
			return
		case "delete":
			deleteConfig()
			return
		case "switch":
			switchProfile()
			return
		case "list":
			listProfiles()
			return
		default:
			log.Fatal("Unknown command")
		}
		return

	case "upgrade":
		upgrade()
		return
	case "list":
		listMods()
		return
	case "search":
		err := searchMod.Parse(os.Args[2:])
		checkError(err)
		modName := os.Args[2]
		Search(modName, backward)
	case "help":
		displaySimpleText(helpStrings)
	case "testing":
		return
	default:
		log.Fatal("Unknown command")
	}
}

func displaySimpleText(stringsToDisplay []string) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Println("Error getting terminal size:", err)
		return
	}
	boxWidth := width - 2
	if boxWidth < 1 {
		boxWidth = 1
	}

	boxHeight := len(stringsToDisplay) + 2
	if boxHeight > height-2 {
		boxHeight = height - 2
	}

	// Top
	fmt.Printf("┌%s┐\n", strings.Repeat("─", boxWidth))

	verticalPadding := (boxHeight - len(stringsToDisplay)) / 2

	for i := 0; i < verticalPadding; i++ {
		fmt.Printf("│%s│\n", strings.Repeat(" ", boxWidth))
	}

	// Text
	for _, line := range stringsToDisplay {
		if len(line) > boxWidth {
			line = line[:boxWidth]
		}

		leftPadding := (boxWidth - len(line)) / 2
		rightPadding := boxWidth - len(line) - leftPadding
		if (strings.Contains(line, "//") || strings.Contains(line, "\\")) && !strings.Contains(line, "gpl") { // Don't look, it's done very poorly.
			rightPadding++
		}

		fmt.Printf("│%s%s%s│\n", strings.Repeat(" ", leftPadding), line, strings.Repeat(" ", rightPadding))
	}

	// Empty lines
	for i := 0; i < boxHeight-len(stringsToDisplay)-verticalPadding; i++ {
		fmt.Printf("│%s│\n", strings.Repeat(" ", boxWidth))
	}

	// Bottom
	fmt.Printf("└%s┘\n", strings.Repeat("─", boxWidth))
}

//	Function for fetching latest version

func fetchLatestVersion(modName string, gameVersion string, loader string, backward []bool) *Version {

	urlProject := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version", modName)

	client := http.Client{
		Timeout: time.Second * 5,
	}

	req, _ := http.NewRequest("GET", urlProject, nil)
	req.Header.Set("User-Agent", FullVersion)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(Red + "Error fetching the latest version" + Reset)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		checkError(err)
	}(resp.Body)

	body, _ := io.ReadAll(resp.Body)

	var versions []Version
	err = json.Unmarshal(body, &versions)
	checkError(err)

	var filteredVersions []Version

	for _, version := range versions {
		if !backward[1] && !backward[0] {
			if slices.Contains(version.GameVersions, gameVersion) && slices.Contains(version.Loaders, loader) {
				filteredVersions = append(filteredVersions, version)
			}
		} else {
			if backward[1] && slices.Contains(version.GameVersions, gameVersion) && (slices.Contains(version.Loaders, loader) || slices.Contains(version.Loaders, "forge")) {
				filteredVersions = append(filteredVersions, version)
			}
			if backward[0] && slices.Contains(version.GameVersions, gameVersion) && (slices.Contains(version.Loaders, loader) || slices.Contains(version.Loaders, "fabric")) {
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

// function to download file from url
func downloadFile(url string, modsPath string, filename string) error {
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading the file: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		checkError(err)
	}(response.Body)

	if !dirExists(modsPath) {
		err := os.Mkdir(modsPath, 0755)
		checkError(err)
	}

	file, err := os.Create(path.Join(modsPath, filename))
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		checkError(err)
	}(file)

	fmt.Printf("[Downloading] [%s%s%s]\n", Cyan, filename, Reset)

	_, err = io.Copy(file, response.Body)
	checkError(err)

	return nil
}

func downloadFilesConcurrently(modsPath string, urls []map[string]string) {
	var wg sync.WaitGroup
	wg.Add(len(urls))

	for _, urlMap := range urls {
		go func(urlMap map[string]string) {
			defer wg.Done()
			if err := downloadFile(urlMap["url"], modsPath, urlMap["filename"]); err != nil {
				log.Printf("%sError: %s%s", Red, err.Error(), Reset)
			}
		}(urlMap)
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
	folder, mineVersion, loader, name, hash := getConfigDataToWrite()

	newConfig := Config{
		ModsFolder:  path.Join(folder, ""),
		GameVersion: mineVersion,
		Loader:      loader,
		Name:        name,
		Active:      "*",
		Hash:        hash,
	}

	configPath, _ := getConfigPath()

	oldConfig := readFullConfig(configPath)

	for i := range oldConfig.Profiles {
		oldConfig.Profiles[i].Active = ""
	}

	oldConfig.Profiles = append(oldConfig.Profiles, newConfig)

	jsonData, _ := json.MarshalIndent(oldConfig, "", "  ")

	err := os.WriteFile(configPath, jsonData, 0644)
	checkError(err)
}

func getConfigDataToWrite() (string, string, string, string, string) {
	var folder string
	var mineVersion string
	var loader string
	var name string
	var hash string
	for i := 0; i < 4; {
		switch i {
		case 0:
			fmt.Print("Enter mods folder path: ")
			_, err := fmt.Scanln(&folder)
			checkError(err)
			if dirExists(folder) {
				i = 1
			}
		case 1:
			fmt.Print("Enter Minecraft version: ")
			_, err := fmt.Scanln(&mineVersion)
			checkError(err)
			if mineVersion != "" {
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
			_, err := fmt.Scanln(&name)
			checkError(err)
			if name != "" {
				i = 4
			}
		}
	}
	hash = generateRandomHash()
	return folder, mineVersion, loader, name, hash
}

func readConfig(path string) Config {
	configFile, _ := os.ReadFile(path)

	var config MultiConfig
	err := json.Unmarshal(configFile, &config)
	checkError(err)
	for _, rootConfig := range config.Profiles {
		if rootConfig.Active == "*" {
			return rootConfig
		}
	}
	return Config{}
}

func readFullConfig(path string) MultiConfig {
	configFile, _ := os.ReadFile(path)

	var config MultiConfig
	err := json.Unmarshal(configFile, &config)
	checkError(err)
	return config
}

func deleteConfig() {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Printf("%sNo profile found to delete%s", Red, Reset)
		return
	}
	configData := readFullConfig(configPath)
	menu := cli.NewMenu("Select the profile you want to delete")
	for _, profile := range configData.Profiles {
		if profile.Active == "*" {
			menu.AddItem(fmt.Sprintf("%s %s[%s%s%s] [%s%s%s, %s%s%s] [%s%s%s]", profile.Name, Reset, Green, "Active", Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset), profile.Hash)
		} else {
			menu.AddItem(fmt.Sprintf("%s %s[%s%s%s, %s%s%s] [%s%s%s]", profile.Name, Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset), profile.Hash)
		}
	}
	selectedProfile := menu.Display()
	var newConfig MultiConfig
	needToChooseNewProfile := false
	for i := range configData.Profiles {
		if !(selectedProfile == configData.Profiles[i].Hash) {
			newConfig.Profiles = append(newConfig.Profiles, configData.Profiles[i])
		}
		if selectedProfile == configData.Profiles[i].Hash && configData.Profiles[i].Active == "*" {
			needToChooseNewProfile = true
		}
	}

	if needToChooseNewProfile {
		if len(newConfig.Profiles) == 0 {
			jsonData, _ := json.MarshalIndent(newConfig, "", "  ")
			err := os.WriteFile(configPath, jsonData, 0644)
			checkError(err)
			return
		}
		if len(newConfig.Profiles) == 1 {
			for i := range newConfig.Profiles {
				newConfig.Profiles[i].Active = "*"
			}
			jsonData, _ := json.MarshalIndent(newConfig, "", "  ")
			err := os.WriteFile(configPath, jsonData, 0644)
			checkError(err)
			return
		}
		secondMenu := cli.NewMenu("Select profile to switch to")
		for _, profile := range newConfig.Profiles {
			secondMenu.AddItem(fmt.Sprintf("%s %s[%s%s%s, %s%s%s] [%s%s%s]", profile.Name, Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset), profile.Hash)
		}
		selectedProfile = secondMenu.Display()
		for i := range newConfig.Profiles {
			fmt.Println(selectedProfile, newConfig.Profiles[i].Hash)
			if selectedProfile == newConfig.Profiles[i].Hash {
				newConfig.Profiles[i].Active = "*"
			}
		}
	}
	jsonData, _ := json.MarshalIndent(newConfig, "", "  ")
	err := os.WriteFile(configPath, jsonData, 0644)
	checkError(err)
	return
}

func getConfigPath() (string, string) {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".config", "gorium", "config.json"), path.Join(home, ".config", "gorium")
}

func generateRandomHash() string {
	randomBytes := make([]byte, 64)
	_, err := rand.Read(randomBytes)
	checkError(err)

	hash := sha512.New()
	hash.Write(randomBytes)

	hashString := hex.EncodeToString(hash.Sum(nil))
	return hashString
}

func getSHA512HashesFromDirectory(dir string) []string {
	var hashes []string

	files, err := os.ReadDir(dir)
	checkError(err)

	for _, file := range files {
		if !file.IsDir() {
			filePath := path.Join(dir, file.Name())
			hash := hashFileSHA512(filePath)
			hashes = append(hashes, hash)
		}
	}

	return hashes
}

// function to calculate SHA512 from file
func hashFileSHA512(filePath string) string {
	file, err := os.Open(filePath)
	checkError(err)
	defer func(file *os.File) {
		err := file.Close()
		checkError(err)
	}(file)

	hash := sha512.New()
	_, err = io.Copy(hash, file)
	checkError(err)

	hashInBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)
	return hashString
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func upgrade() {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Printf("%sNo profile found to upgrade%s", Red, Reset)
		return
	}
	configData := readConfig(configPath)
	if len(configData.Name) == 0 {
		fmt.Printf("%sNo profile found to upgrade%s", Red, Reset)
		return
	}

	modsPath := configData.ModsFolder
	loader := configData.Loader
	gameVersion := configData.GameVersion

	hashes := getSHA512HashesFromDirectory(modsPath)

	if len(hashes) < 1 {
		fmt.Println("There's no mods, type gorium add")
		return
	}

	loaderList := []string{loader}

	switch loader {
	case "quilt":
		loaderList = append(loaderList, "fabric")
	case "neoforge":
		loaderList = append(loaderList, "forge")
	default:
	}

	data := HashesToSend{
		Hashes:    hashes,
		Algorithm: "sha512",
		Loaders:   loaderList,
		GameVersions: []string{
			gameVersion,
		},
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")

	url := "https://api.modrinth.com/v2/version_files/update"
	url2 := "https://api.modrinth.com/v2/version_files"

	r := bytes.NewReader(jsonData)
	r2 := bytes.NewReader(jsonData)

	req, _ := http.NewRequest("POST", url, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", FullVersion)

	client := &http.Client{}
	resp, err := client.Do(req)
	checkError(err)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		checkError(err)
	}(resp.Body)

	body, _ := io.ReadAll(resp.Body)

	req2, _ := http.NewRequest("POST", url2, r2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("User-Agent", FullVersion)

	resp2, err := client.Do(req2)
	checkError(err)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		checkError(err)
	}(resp2.Body)

	body2, _ := io.ReadAll(resp2.Body)

	var rootMap map[string]Root
	var rootMap2 map[string]Root
	err = json.Unmarshal(body, &rootMap)
	checkError(err)
	err = json.Unmarshal(body2, &rootMap2)
	checkError(err)

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
					if dirExists(path.Join(modsPath, file3.Filename)) {
						err := os.Remove(path.Join(modsPath, file3.Filename))
						checkError(err)
					}
				}
			}
		}
	}

	if len(fileList) == 0 {
		fmt.Println("No updates found")
		return
	}

	downloadFilesConcurrently(modsPath, fileList)
	fmt.Printf("%sUpgrade completed succesfully%s", Green, Reset)
	return
}

func switchProfile() {
	configPath, _ := getConfigPath()
	roots := readFullConfig(configPath)
	switchMenu := cli.NewMenu("Choose profile")
	for _, profile := range roots.Profiles {
		if profile.Active == "*" {
			switchMenu.AddItem(fmt.Sprintf("%s %s[%s%s%s] [%s%s%s, %s%s%s] [%s%s%s]", profile.Name, Reset, Green, "Active", Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset), profile.Hash)
		} else {
			switchMenu.AddItem(fmt.Sprintf("%s %s[%s%s%s, %s%s%s] [%s%s%s]", profile.Name, Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset), profile.Hash)
		}
	}
	if len(switchMenu.MenuItems) < 2 {
		fmt.Println("No profiles to switch")
		return
	}
	chosenProfile := switchMenu.Display()
	for i := range roots.Profiles {
		if roots.Profiles[i].Active == "*" {
			roots.Profiles[i].Active = ""
		}
		if roots.Profiles[i].Hash == chosenProfile {
			roots.Profiles[i].Active = "*"
		}
	}

	jsonData, _ := json.MarshalIndent(roots, "", "  ")

	err := os.WriteFile(configPath, jsonData, 0644)
	checkError(err)
	return
}

func listProfiles() {
	configPath, _ := getConfigPath()
	roots := readFullConfig(configPath)
	if len(roots.Profiles) < 1 {
		fmt.Println("No profiles to list")
		return
	}
	for _, profile := range roots.Profiles {
		if profile.Active == "*" {
			fmt.Printf("%s [%s%s%s] [%s%s%s, %s%s%s] [%s%s%s]\n", profile.Name, Green, "Active", Reset, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset)
		} else {
			fmt.Printf("%s [%s%s%s, %s%s%s] [%s%s%s]\n", profile.Name, Cyan, profile.Loader, Reset, Yellow, profile.GameVersion, Reset, White, profile.ModsFolder, Reset)
		}
	}
}

func listMods() {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
		return
	}
	url := "https://api.modrinth.com/v2/version_files"

	configData := readConfig(configPath)
	modsFolder := configData.ModsFolder
	gameVersion := configData.GameVersion
	loader := configData.Loader
	hashes := getSHA512HashesFromDirectory(modsFolder)

	if len(hashes) < 1 {
		fmt.Println("There's no mods, type gorium add")
		return
	}

	data := HashesToSend{
		Hashes:    hashes,
		Algorithm: "sha512",
		Loaders: []string{
			loader,
		},
		GameVersions: []string{
			gameVersion,
		},
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")

	r := bytes.NewReader(jsonData)

	req, _ := http.NewRequest("POST", url, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", FullVersion)

	client := &http.Client{}
	resp, _ := client.Do(req)

	body, _ := io.ReadAll(resp.Body)

	var rootMap map[string]Root

	err := json.Unmarshal(body, &rootMap)
	checkError(err)

	i := 1

	for _, root := range rootMap {
		fmt.Printf("[%d] %s (%s) \n", i, root.Name, root.Files)
		i += 1
	}

	return
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func Search(modName string, backward []bool) {
	configPath, _ := getConfigPath()
	if !dirExists(configPath) {
		fmt.Println(Red + "No profile found, type gorium profile create" + Reset)
		return
	}
	configData := readConfig(configPath)

	loader := configData.Loader
	version := configData.GameVersion
	modsPath := configData.ModsFolder

	urlProject := fmt.Sprintf("https://api.modrinth.com/v2/search?query=%s&limit=100", modName)
	client := http.Client{
		Timeout: time.Second * 5,
	}

	req, _ := http.NewRequest("GET", urlProject, nil)
	req.Header.Set("User-Agent", FullVersion)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(Red + "Error fetching search results" + Reset)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		checkError(err)
	}(resp.Body)

	body, _ := io.ReadAll(resp.Body)
	var results SearchRoot
	err = json.Unmarshal(body, &results)
	checkError(err)

	var sortedResults SearchRoot

	for _, hit := range results.Hits {
		if hit.ProjectType == "mod" && contains(hit.Versions, version) {
			if backward[1] {
				if contains(hit.Categories, loader) || contains(hit.Categories, "forge") {
					sortedResults.Hits = append(sortedResults.Hits, hit)
				}
			} else if backward[0] {
				if contains(hit.Categories, loader) || contains(hit.Categories, "fabric") {
					sortedResults.Hits = append(sortedResults.Hits, hit)
				}
			} else {
				if contains(hit.Categories, loader) {
					sortedResults.Hits = append(sortedResults.Hits, hit)
				}
			}
		}
	}

	if len(sortedResults.Hits) == 0 {
		fmt.Println(Red + "No results found" + Reset)
		return
	}

	for i := len(sortedResults.Hits) - 1; i >= 0; i-- {
		fmt.Printf("[%d] %s\n", i+1, sortedResults.Hits[i].Title)
	}

	in := bufio.NewReader(os.Stdin)

	fmt.Print("Enter numbers of mods you want to install: ")

	selected, _ := in.ReadString('\n')

	selected = strings.TrimSpace(selected)

	substrings := strings.Split(selected, " ")

	var selectedIntegers []int
	for _, oneString := range substrings {
		str, _ := strconv.Atoi(oneString)
		selectedIntegers = append(selectedIntegers, str-1)
	}

	var modsToDownload []string
	for i := range selectedIntegers {
		modsToDownload = append(modsToDownload, sortedResults.Hits[selectedIntegers[i]].ProjectID)
	}

	type Versions struct {
		Version []*Version
	}
	var latestVersions Versions
	for i := range modsToDownload {
		latestVersions.Version = append(latestVersions.Version, fetchLatestVersion(modsToDownload[i], version, loader, backward))
	}

	var filesToDownload []map[string]string
	for _, root := range latestVersions.Version {
		for _, file := range root.Files {
			fileInfo := map[string]string{
				"url":      file.URL,
				"filename": file.Filename,
			}
			if !strings.Contains(fileInfo["filename"], "sources") {
				filesToDownload = append(filesToDownload, fileInfo)
			}
		}
	}
	downloadFilesConcurrently(modsPath, filesToDownload)
}
