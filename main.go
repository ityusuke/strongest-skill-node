package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/bluele/mecab-golang"
	"github.com/kotaroooo0/gojaconv/jaconv"
)

const (
	fetchUrlBase      = "http://hissatuwaza.kill.jp/list/"
	baseSelector      = "#out > table:nth-child(3) > tbody > tr > td:nth-child(2)"
	hiragana          = "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわをん"
	charaTypeHiragana = "hiragana"
	charaTypeKatakana = "katakana"
	charaTypeKanji    = "kanji"
)

func main() {
	var minWordLen = 0
	var charaType string
	var err error

	flag.Parse()
	if len(flag.Args()) >= 1 {
		minWordLen, err = strconv.Atoi(flag.Args()[0])
		if err != nil {
			fmt.Println("failed find minWordLen", err)
		}
	}
	if len(flag.Args()) >= 2 {
		charaType = flag.Args()[1]
	}

	romaAlphabets := genRomaAlphabetKanas()
	skillWords, err := fetchSkillsByCrawling(romaAlphabets)
	if err != nil {
		fmt.Println("failed fetch skills by crawling", err)
	}
	nounMap, err := parseToNode(skillWords, minWordLen, charaType)
	if err != nil {
		fmt.Println("failed parse to node", err)
	}
	// ランキング表示
	res := sortedKeys(nounMap)
	for i, v := range res {
		fmt.Println(fmt.Sprintf(`第%d位: %s %d回`, i+1, v, nounMap[v]))
		if i >= 9 {
			return
		}
	}
}

func parseToNode(skillWords []string, minWordLen int, charaType string) (map[string]int, error) {
	var nounNodes []string
	nounMap := make(map[string]int)

	m, err := mecab.New("-Owakati")
	if err != nil {
		return nil, errors.New("failed init mecab")
	}
	defer m.Destroy()

	tg, err := m.NewTagger()
	if err != nil {
		return nil, errors.New("failed init tagger")
	}
	defer tg.Destroy()
	for _, w := range skillWords {
		lt, err := m.NewLattice(w)
		if err != nil {
			return nil, errors.New("failed init lattice")
		}
		defer lt.Destroy()

		node := tg.ParseToNode(lt)
		for {
			features := strings.Split(node.Feature(), ",")
			if features[0] == "名詞" && utf8.RuneCountInString(node.Surface()) >= minWordLen && isMatchCharaType(charaType, node.Surface()) {
				nounNodes = append(nounNodes, node.Surface())
			}
			if node.Next() != nil {
				break
			}
		}
		for _, n := range nounNodes {
			if _, ok := nounMap[n]; ok {
				nounMap[n]++
			} else {
				nounMap[n] = 1
			}
		}
	}
	return nounMap, nil
}

func fetchSkillsByCrawling(romaAlphabets []string) ([]string, error) {
	skillWords := make([]string, len(romaAlphabets))
	for _, ra := range romaAlphabets {
		fetchUrl := fetchUrlBase + ra + ".htm" // ex. http://hissatuwaza.kill.jp/list/ki.htm
		resp, err := http.Get(fetchUrl)
		if err != nil {
			return nil, errors.New("failed get response from " + fetchUrl)
		}
		defer resp.Body.Close()
		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			return nil, errors.New("failed get document from " + fetchUrl)
		}
		var skillWord string
		doc.Find(baseSelector).Each(func(i int, s *goquery.Selection) {
			s.Find("a").Each(func(j int, y *goquery.Selection) {
				// 各必殺技の()内の読みを除去
				skillWord += " " + regexp.MustCompile(`[(（].*[）)]`).ReplaceAllString(y.Text(), "")
			})
		})
		skillWords = append(skillWords, skillWord)
	}
	return skillWords, nil
}

func genRomaAlphabetKanas() []string {
	kanaChars := strings.Split(hiragana, "")
	romaAlphabets := make([]string, len(kanaChars))
	for i, kc := range kanaChars {
		switch kc {
		case "ち":
			romaAlphabets[i] = jaconv.ToHebon("ti")
			continue
		case "つ":
			romaAlphabets[i] = jaconv.ToHebon("tu")
			continue
		case "ふ":
			romaAlphabets[i] = jaconv.ToHebon("hu")
			continue
		case "を":
			romaAlphabets[i] = jaconv.ToHebon("wo")
			continue
		case "ん":
			romaAlphabets[i] = jaconv.ToHebon("nn")
			continue
		}
		romaAlphabets[i] = jaconv.ToHebon(kc)
	}
	return romaAlphabets
}

type sortedMap struct {
	m map[string]int
	s []string
}

func sortedKeys(m map[string]int) []string {
	sm := new(sortedMap)
	sm.m = m
	sm.s = make([]string, len(m))
	i := 0
	for key, _ := range m {
		sm.s[i] = key
		i++
	}
	sort.Sort(sm)
	return sm.s
}

func (sm *sortedMap) Len() int {
	return len(sm.m)
}
func (sm *sortedMap) Less(i, j int) bool {
	return sm.m[sm.s[i]] > sm.m[sm.s[j]]
}
func (sm *sortedMap) Swap(i, j int) {
	sm.s[i], sm.s[j] = sm.s[j], sm.s[i]
}

func isMatchCharaType(charaType string, skill string) bool {
	for _, rune := range []rune(skill) {
		switch charaType {
		case charaTypeHiragana:
			if !unicode.In(rune, unicode.Hiragana) {
				return false
			}
			break
		case charaTypeKatakana:
			if !unicode.In(rune, unicode.Katakana) {
				return false
			}
			break
		case charaTypeKanji:
			if !unicode.In(rune, unicode.Han) {
				return false
			}
		}
	}
	return true
}
