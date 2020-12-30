package core

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	wordSegment = regexp.MustCompile(`[#@]?[\pL\p{Mc}\p{Mn}_']+`)
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
	StopWords []string       `json:"stop_words"`
	Ids       map[string]Ids `json:"ids"`
}

func SearchTask(project Project, board string, matchAll bool,  keys ...string) ([]TaskInfo, error) {
	infos, err := ListTasks(project, board, "")
	if IsErr(err, "cannot list tasks during search in %s/%s", project.Path, board) {
		return []TaskInfo{}, err
	}

	if len(keys) == 0 {
		return infos, nil
	}

	idsSet, err := lookupTaskIds(project, keys...)
	if IsErr(err, "cannot lookup ids on keys %v during search in %s/%s", keys, project.Path, board) {
		return []TaskInfo{}, err
	}

	l := len(infos)
	for i:=0 ; i < l; {
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

func SearchTaskIds(project Project, keys ...string) (Ids, error) {
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

func lookupTaskIds(project Project, keys ...string) (map[uint16]int, error) {
	if project.Index == nil {
		index, _, err := ReadIndex(project)
		if IsErr(err, "cannot read index for %s", project.Path) {
			return map[uint16]int{}, err
		}
		project.Index = index
	}

	idsSet := make(map[uint16]int)
	for _, key := range keys {
		ids, ok := project.Index.Ids[key]
		if ok {
			for _, id := range ids {
				logrus.Debugf("Found key %s in task %d", key, id)
				idsSet[id] += 1
			}
		}
	}
	return idsSet, nil
}

func ClearIndex(project Project) error {
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

func ReIndex(project Project) error {
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
	idsLimit := len(infos) / 20
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

	for _, word := range newStopWords {
		delete(index.Ids, word)
	}
	index.StopWords = append(index.StopWords, newStopWords...)
	project.Index = index
	return WriteIndex(project, index)
}

func clearIndex(id uint16, index *Index) {
	for key, ids := range index.Ids {
		l := len(ids)
		logrus.Debugf("Word %s has %d references: %v", key, l, ids)
		for i := 0; i < l; {
			if ids[i] == id {
				logrus.Debugf("Remove reference to task %d in word %s", id, key)
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
		logrus.Debugf("After clean-up word %s has %d references: %v", key, l, ids)
	}
}

func mergeToIndex(id uint16, words []string, index *Index, limit int, newStopWords *[]string) {
	for _, word := range words {
		if ids, found := index.Ids[word]; found {
			if isMissing(id, ids) {
				if limit > 0 && len(ids) > limit && newStopWords != nil {
					*newStopWords = append(*newStopWords, word)
					logrus.Debugf("Add word %s to Stop Words", word)
				} else {
					index.Ids[word] = append(ids, id)
					logrus.Debugf("Add reference to task %d in word %s", id, word)
				}
			}
		} else {
			index.Ids[word] = Ids{id}
			logrus.Debugf("Add reference to task %d in word %s", id, word)
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

func indexTask(project Project, board string, path string) (normal []string, special []string) {
	p := filepath.Join(project.Path, "boards", board, path+TaskFileExt)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		logrus.Errorf("Cannot read task file %s: %v", p, err)
		return []string{}, []string{}
	}

	return cleanText(data)
}

func cleanText(text []byte) (normal []string, special []string) {
	normal = make([]string, 0)
	special = make([]string, 0)

	text = norm.NFC.Bytes(text)
	text = bytes.ToLower(text)
	words := wordSegment.FindAll(text, -1)
	for _, w := range words {
		s := string(w)
		if _, found := stopWords[s]; !found {
			if s[0] == '@' || s[0] == '#' {
				special = append(special, s)
			} else {
				normal = append(normal, s)
			}
		}
	}
	return normal, special
}

func ReadIndex(project Project) (*Index, time.Time, error) {
	var index = Index{
		StopWords: make([]string, 0),
		Ids:       make(map[string]Ids),
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
	return &index, info.ModTime(), nil
}

func WriteIndex(project Project, index *Index) error {
	p := filepath.Join(project.Path, IndexFile)

	return WriteJSON(p, index)
}