package core

import (
	"github.com/monirz/gotri"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	wordSegment = regexp.MustCompile(`[#@]?[\pL\p{Mc}\p{Mn}][\pL\p{Mc}\p{Mn}\p{N}_']*`)
	//	wordSegment = regexp.MustCompile(`([^\n][#@])?[\pNL\p{Mc}\p{Mn}_']+`)
	stopWords = english
)

// TagLink is a link to a story
type TagLink struct {
	Name  string `json:"n"`
	Board string `json:"b"`
	Path  string `json:"p"`
}

// TagLinks is the list of links for a tag
type TagLinks []TagLink

type Ids []uint16

type Index struct {
	StopWords  []string       `json:"stop_words"`
	Ids        map[string]Ids `json:"ids"`
	searchTree *gotri.Trie
}

func SearchTask(project *Project, board string, matchAll bool, keys ...string) ([]TaskInfo, error) {
	infos, err := ListTasks(project, board, "")
	if IsErr(err, "cannot list tasks during search in %s/%s", project.Path, board) {
		return []TaskInfo{}, err
	}
	logrus.Infof("Found %d tasks in board %s", len(infos), board)

	if len(keys) == 0 {
		return infos, nil
	}

	idsSet, err := lookupTaskIds(project, keys...)
	if IsErr(err, "cannot lookup ids on keys %v during search in %s/%s", keys, project.Path, board) {
		return []TaskInfo{}, err
	}
	logrus.Infof("Found %d tasks with keys %v: %v", len(idsSet), keys, idsSet)

	l := len(infos)
	for i := 0; i < l; {
		cnt := idsSet[infos[i].ID]
		logrus.Infof("Task %s/%s matches on %d keys", infos[i].Board, infos[i].Name, cnt)
		if matchAll && cnt < len(keys) || cnt == 0 {
			logrus.Infof("Task %s/%s removed from search output", infos[i].Board, infos[i].Name)
			infos[i] = infos[l-1]
			l -= 1
		} else {
			logrus.Infof("Task %s/%s match in search output", infos[i].Board, infos[i].Name)
			i += 1
		}
	}
	return infos[0:l], nil
}


func SuggestKeys(project *Project, prefix string, total int) []string {
	suggestions := project.Index.searchTree.GetSuggestion(prefix, total)
	if suggestions == nil {
		return []string{}
	}
	return suggestions
}

func SearchTaskIds(project *Project, keys ...string) (Ids, error) {
	idsSet, err := lookupTaskIds(project, keys...)
	if IsErr(err, "cannot lookup ids on keys %v", keys) {
		return Ids{}, err
	}
	ids := make(Ids, 0, len(idsSet))
	for id := range idsSet {
		ids = append(ids, id)
	}
	return ids, nil
}

func lookupTaskIds(project *Project, keys ...string) (map[uint16]int, error) {
	if project.Index == nil {
		index, _, err := ReadIndex(project)
		if IsErr(err, "cannot read index for %s", project.Path) {
			return map[uint16]int{}, err
		}
		project.Index = index
	}

	idsSet := make(map[uint16]int)
	for _, key := range keys {
		if !strings.HasPrefix(key, "@") && !strings.HasPrefix(key, "#") {
			key = strings.ToLower(key)
		}
		ids, ok := project.Index.Ids[key]
		if ok {
			for _, id := range ids {
				idsSet[id] += 1
			}
		}
	}
	return idsSet, nil
}

func ClearIndex(project *Project) error {
	p := filepath.Join(project.Path, IndexFile)
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	project.Index = nil
	return os.Remove(p)
}

func containId(ids Ids, id uint16) bool {
	for _, _id := range ids {
		if _id == id {
			return true
		}
	}
	return false
}

func diffIds(a Ids, b Ids) (removed Ids, added Ids) {
	removed = Ids{}
	added = Ids{}

	for _, id := range a {
		if !containId(b, id) {
			removed = append(removed, id)
		}
	}
	for _, id := range b {
		if !containId(a, id) {
			added = append(added, id)
		}
	}
	return
}

func ReIndex(project *Project) error {
	logrus.Debugf("Reindex project %s", project.Path)

	index, modTime, err := ReadIndex(project)
	if err != nil {
		return err
	}

	infos, err := ListTasks(project, "", "")
	if err != nil {
		return err
	}

	mergeStopWords(index)
	newStopWords := make([]string, 0)
	idsLimit := len(infos) / 10
	if idsLimit < 10 {
		idsLimit = 10
	}

	for _, info := range infos {
		if info.ModTime.Sub(modTime) > 0 {
			logrus.Debugf("ReIndex task %s/%s", info.Board, info.Name)
			normal, special := indexTask(project, info.Board, info.Name)
			clearIndex(info.ID, index)
			mergeToIndex(info.ID, normal, index, idsLimit, &newStopWords)
			mergeToIndex(info.ID, special, index, -1, nil)
		}
	}

	logrus.Debugf("New stop words: %v", newStopWords)
	for _, word := range newStopWords {
		delete(index.Ids, word)
	}
	index.StopWords = append(index.StopWords, newStopWords...)
	project.Index = index

	if logrus.GetLevel() == logrus.DebugLevel {
		oldIndex, _, err := ReadIndex(project)
		if err == nil {
			for key, value := range index.Ids {
				oldValue, found := oldIndex.Ids[key]
				if found {
					removed, added := diffIds(oldValue, value)
					if len(added) > 0 || len(removed) > 0 {
						logrus.Debugf("Index %s has been updated: %v\n"+
							"Added: %v\nRemoved: %v\n",
							key, value, added, removed)

					}
					if len(added) > 0 {
						logrus.Debugf("Added references: %v", removed)
					}
					if len(removed) > 0 {
						logrus.Debugf("Removed references: %v", removed)
					}
				} else {
					logrus.Debugf("New index %s: %v", key, value)
				}
			}

		}

	}

	BuiltSearchTree(project)
	return WriteIndex(project, index)
}

func clearIndex(id uint16, index *Index) {
	for key, ids := range index.Ids {
		l := len(ids)
		for i := 0; i < l; {
			if ids[i] == id {
				ids[i] = ids[l-1]
				l -= 1
			} else {
				i += 1
			}
		}
		if l != len(ids) {
			ids = ids[0:l]
			index.Ids[key] = ids
		}
	}
}

func mergeToIndex(id uint16, words []string, index *Index, limit int, newStopWords *[]string) {
	for _, word := range words {
		if ids, found := index.Ids[word]; found {
			if isMissing(id, ids) {
				if limit > 0 && len(ids) > limit && newStopWords != nil {
					found := false
					for _, w := range *newStopWords {
						if w == word {
							found = true
						}
					}
					if !found {
						*newStopWords = append(*newStopWords, word)
					}
				} else {
					index.Ids[word] = append(ids, id)
				}
			}
		} else {
			index.Ids[word] = Ids{id}
		}
	}
}

func isMissing(id uint16, ids Ids) bool {
	for _, _id := range ids {
		if _id == id {
			return false
		}
	}
	return true
}

func mergeStopWords(index *Index) {
	for _, word := range index.StopWords {
		stopWords[word] = ""
	}
}

func indexTask(project *Project, board string, name string) (normal []string, special []string) {
	p := filepath.Join(project.Path, "boards", board, name+TaskFileExt)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		logrus.Errorf("Cannot read task file %s: %v", p, err)
		return []string{}, []string{}
	}
	_, title := ExtractTaskId(name)
	data = append(data, []byte(title)...)

	normal, special = cleanText(data)
	logrus.Debugf("Indexing %s/%s\n Normal Words: %v\n Special Words %v\n",
		board, name, normal, special)
	return
}

func cleanText(text []byte) (normal []string, special []string) {
	normal = make([]string, 0)
	special = make([]string, 0)

	text = norm.NFC.Bytes(text)
	words := wordSegment.FindAll(text, -1)
	for _, w := range words {
		s := string(w)
		if _, found := stopWords[s]; !found {
			if s[0] == '@' || s[0] == '#' {
				special = append(special, s)
			} else {
				normal = append(normal, strings.ToLower(s))
			}
		}
	}
	return normal, special
}

func BuiltSearchTree(project *Project) {
	for k := range project.Index.Ids {
		project.Index.searchTree.Add(k, k)
	}
	for _, propertyDef := range project.Config.PropertyModel {
		if propertyDef.Kind == "Tag" && propertyDef.Values != nil {
			for _, value := range propertyDef.Values {
				project.Index.searchTree.Add(value, value)
			}
		}
	}
}

func ReadIndex(project *Project) (*Index, time.Time, error) {
	var index = Index{
		StopWords:  make([]string, 0),
		Ids:        make(map[string]Ids),
		searchTree: new(gotri.Trie),
	}
	p := filepath.Join(project.Path, IndexFile)
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return &index, time.Unix(0, 0), nil
	}
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	err = ReadJSON(p, &index)
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	project.Index = &index
	BuiltSearchTree(project)
	return &index, info.ModTime(), nil
}

func WriteIndex(project *Project, index *Index) error {
	p := filepath.Join(project.Path, IndexFile)

	return WriteJSON(p, index)
}
