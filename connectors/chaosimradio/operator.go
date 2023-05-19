package chaosimradio

/* pad2gh is a simple tool to get the first link from https://pad.ccc-p.org/Radio, extract the information from the markdown text and create a github PR with the information */

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"gopkg.in/yaml.v3"
)

type OpCiR struct {
}

var _ base.FreepsOperator = &OpCiR{}

// CiRaudio is the audio information for the podcast
type CiRaudio struct {
	Url      string // format: https://cdn.ccc-p.org/episodes/2021-01-01-episode.mp3 `yaml:"url"`
	MimeType string // format: audio/mpeg `yaml:"mimeType"`
}

// CiRChapter is the chapter information for the podcast
type CiRChapter struct {
	Start string // format: 00:00:00.000 `yaml:"start"`
	Title string `yaml:"title"`
}

// CiREntry is the podcast episode information
type CiREntry struct {
	UUID            string       `yaml:"uuid"`
	Title           string       `yaml:"title"`
	Subtitle        string       `yaml:"subtitle"`
	Summary         string       `yaml:"summary"`
	PublicationDate string       `yaml:"publicationDate"`
	Audio           CiRaudio     `yaml:"audio"`
	Chapters        []CiRChapter `yaml:"chapters"`
	LongSummaryMD   string       `yaml:"long_summary_md"`
}

func getPadContent(padURL string) (io.ReadCloser, error) {
	// append the HedgeDoc API path to get the raw pad content
	padURL = strings.TrimSuffix(padURL, "/")
	padURL = fmt.Sprintf("%s/download", padURL)
	resp, err := http.Get(padURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pad url must be accessible")
	}
	return resp.Body, nil
}

func getTitleFromFMA(fmaURL string) (string, error) {
	// append the HedgeDoc API path to get the raw pad content
	resp, err := http.Get(fmaURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fma url must be accessible")
	}
	defer resp.Body.Close()
	// find the title tag in the html and return the content
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "<title>") {
			return strings.TrimSuffix(strings.TrimPrefix(line, "<title>"), "</title>"), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return fmaURL, nil
}

func getFirstLink(padURL string) (string, error) {
	padContent, err := getPadContent(padURL)
	if err != nil {
		return "", err
	}

	defer padContent.Close()

	// parse the content to find the first link
	scanner := bufio.NewScanner(padContent)
	for scanner.Scan() {
		line := scanner.Text()
		for _, linkCandidate := range strings.Split(line, "(") {
			if strings.HasPrefix(linkCandidate, "https://pad.ccc-p.org/") {
				if strings.HasSuffix(linkCandidate, ")") {
					link := strings.Split(linkCandidate, ")")
					if len(link) > 1 {
						return link[0], nil
					}
				}
			}
		}

	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func findFirstLink(line string) string {
	for _, linkCandidate := range strings.Split(line, " ") {
		if strings.HasPrefix(linkCandidate, "http") {
			return linkCandidate
		}
	}
	return ""
}

func getMarkdownContentBySection(padURL string) (map[string][]string, error) {
	padContent, err := getPadContent(padURL)
	if err != nil {
		return nil, err
	}
	defer padContent.Close()

	// parse the content to find the first link
	scanner := bufio.NewScanner(padContent)
	currentSection := "pre-section"
	currentSectionContent := []string{}
	contentBySection := make(map[string][]string)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") {
			contentBySection[strings.ToLower(currentSection)] = currentSectionContent
			currentSectionContent = []string{}
			currentSection = strings.TrimPrefix(line, "## ")
			continue
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		currentSectionContent = append(currentSectionContent, strings.Trim(line, " "))
	}
	contentBySection[currentSection] = currentSectionContent
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return contentBySection, nil
}

type CiROperatorParams struct {
	// URL of the pad to parse
	PadURL              *string `json:"pad_url"`
	AppendToContentFile *bool   `json:"append_to_content_file"`
}

// Pad2ChaosEntry is the main function of the operator
func (cir *OpCiR) Pad2ChaosEntry(ctx *base.Context, mainInput *base.OperatorIO, args CiROperatorParams) *base.OperatorIO {
	padURL := ""
	prComments := []string{}
	var err error

	if args.PadURL != nil {
		padURL = *args.PadURL
		if !strings.HasPrefix(padURL, "https://pad.ccc-p.org/") {
			return base.MakeOutputError(http.StatusBadRequest, "pad url must start with https://pad.ccc-p.org/")
		}
	} else {
		padURL, err = getFirstLink("https://pad.ccc-p.org/Radio")
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
	}
	contentBySection, err := getMarkdownContentBySection(padURL)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	if len(strings.Split(padURL, "_")) < 2 {
		return base.MakeOutputError(http.StatusBadRequest, "pad url must contain a date in the format YYYY-MM-DD_")
	}
	entryDate := strings.Split(padURL, "_")[1]
	if len(entryDate) < 10 {
		return base.MakeOutputError(http.StatusBadRequest, "pad url must contain a date in the format YYYY-MM-DD_")
	}
	year := entryDate[0:4]
	month := entryDate[5:7]
	day := entryDate[8:10]
	longSummary, exists := contentBySection["shownotes"]
	if !exists {
		longSummary, exists = contentBySection["long summary"]
	}
	if !exists {
		return base.MakeOutputError(http.StatusBadRequest, "no shownotes Section in Pad")
	}
	shortSummary, exists := contentBySection["summary"]
	if !exists {
		return base.MakeOutputError(http.StatusBadRequest, "no Summary Section in Pad")
	}

	var entry CiREntry
	entry.UUID = fmt.Sprintf("nt-%s-%s-%s", year, month, day)
	entry.Title = fmt.Sprintf("CiR am %s.%s.%s", day, month, year)
	entry.Subtitle = "Der Chaostreff im Freien Radio Potsdam"
	entry.Summary = strings.Join(shortSummary, "\n")
	entry.PublicationDate = fmt.Sprintf("%s-%s-%sT00:00:00+02:00", year, month, day)
	entry.Audio.Url = fmt.Sprintf("$media_base_url/%s_%s_%s-chaos-im-radio.mp3", year, month, day)
	entry.Audio.MimeType = "audio/mp3"
	entry.LongSummaryMD = "**Shownotes:**\n" + strings.Join(longSummary, "\n")

	chapter, exists := contentBySection["chapters"]
	if exists {
		entry.Chapters = []CiRChapter{}
		for _, c := range chapter {
			chapter := strings.Split(c, " ")
			if len(chapter) < 2 {
				continue
			}
			entry.Chapters = append(entry.Chapters, CiRChapter{Start: chapter[0], Title: strings.Join(chapter[1:], " ")})
		}
	} else {
		prComments = append(prComments, "no chapters Section in Pad")
	}

	mukke, exists := contentBySection["mukke"]
	if exists {
		for _, m := range mukke {
			link := findFirstLink(m)
			if link == "" {
				prComments = append(prComments, fmt.Sprintf("no link found in mukke line: %s", m))
				continue
			}
			title, err := getTitleFromFMA(link)
			if err != nil {
				prComments = append(prComments, fmt.Sprintf("error getting title from fma: %s", err.Error()))
				title = link
			}
			entry.LongSummaryMD = entry.LongSummaryMD + fmt.Sprintf("\n&#x1f3b6;&nbsp;[%s](%s)", title, link)
		}
	} else {
		prComments = append(prComments, "no mukke Section in Pad")
	}

	b, err := yaml.Marshal(entry)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	if args.AppendToContentFile != nil && *args.AppendToContentFile {
		tDir, err := utils.GetTempDir()
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Cannot set working dir: %v", err.Error())
		}
		yappsPath := tDir + "/yaspp"
		contentFilePath := tDir + "/yaspp/content.yaml"
		if _, err := os.Stat(contentFilePath); os.IsNotExist(err) {
			return base.MakeOutputError(http.StatusInternalServerError, "Cannot find yaspp dir: %v", err.Error())
		}
		// append the serialized entry to the content file
		f, err := os.OpenFile(contentFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		{ // block so the file gets properly closed
			defer f.Close()
			if _, err := f.WriteString("---\n"); err != nil {
				return base.MakeOutputError(http.StatusInternalServerError, err.Error())
			}
			if _, err := f.Write(b); err != nil {
				return base.MakeOutputError(http.StatusInternalServerError, err.Error())
			}
		}
		// prepare the git commit and the PR comment
		{
			// get the git commit message
			commitMsg := entry.Title
			if len(prComments) > 0 {
				commitMsg = commitMsg + "\n\n" + strings.Join(prComments, "\n")
			}
			// write the commit message to a file, overwrite the commit-msg file if it exists, create it otherwise
			commitMsgFile, err := os.OpenFile(yappsPath+"/commit-msg", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return base.MakeOutputError(http.StatusInternalServerError, "Error when trying to create commit-msg file: %s", err.Error())
			}
			defer commitMsgFile.Close()
			_, err = commitMsgFile.WriteString(commitMsg)
			if err != nil {
				return base.MakeOutputError(http.StatusInternalServerError, "Error when trying to write commit-msg file: %s", err.Error())
			}
		}
		err = os.Chdir(yappsPath)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error when trying to chdir: %s", err.Error())
		}
		cmd := exec.Command("git", "commit", "-a", "-F", "commit-msg")
		err = cmd.Run()
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error when trying to git add: %s", err.Error())
		}

	}

	return base.MakeByteOutput(b)
}
