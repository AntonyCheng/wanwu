package service

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/bff-service/config"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/response"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/go-ego/riot"
	"github.com/go-ego/riot/types"
)

const (
	docCenterLocalDir        = "configs/microservice/bff-service/static/manual/"
	docCenterStaticAPIPrefix = "../../../user/api/v1/static/manual" // ../../..用于抵消前端固定前缀 aibase/docCenter/pages
	docCenterSnippetLen      = 200                                  // 截取文本长度

	// 文档通用命名：
	// fileName:    e.g. StartNode.md
	// filePath:    configs/microservice/bff-service/static/manual + relFilePath e.g. configs/microservice/bff-service/static/manual/workflow/StartNode.md
	// relFilePath: configs/microservice/bff-service/static/manual中文件的相对路径 e.g. workflow/StartNode.md
)

var (
	mdImageRegex          = regexp.MustCompile(`!\[.*?\]\((.*?)\)`)        // 从markdown文本中匹配 ![](xxxxx) 图片引用
	mdParenthesisRefRegex = regexp.MustCompile(`\((.*?)\)`)                // 从markdown引用中匹配 (xxxxx)
	mdLinkRegex           = regexp.MustCompile(`[^!]\[.*?\]\((.*?\.md)\)`) // 从markdown匹配出跳转链接[](xxxxx)
	mdBracketRegex        = regexp.MustCompile(`\[(.*?)\]`)                // 从markdown匹配[]中的文本

	_docCenter *docCenter
)

type docCenter struct {
	menus    []*response.DocMenu
	contents map[string]string // refFilePath -> content
	searcher *riot.Engine
}

type mdInfo struct {
	relFilePath string
	content     string
}

func InitDocCenter() error {
	if _docCenter != nil {
		return errors.New("already init")
	}

	// 0. 读取docCenterLocalDir所有md文件
	var mdInfos []mdInfo
	if err := filepath.Walk(docCenterLocalDir, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 检查不为目录并且是否为markdown文件
		if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".md") {
			// 读取文件内容
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read %v err: %v", filePath, err)
			}
			// 将markdown文本中图片引用 ![](xxxxx )与链接引用[](xxxxx )里的 xxxxx 处理为前端可访问的地址
			relFilePath := strings.TrimPrefix(filePath, docCenterLocalDir)
			convertByte := convertMarkdown(docCenterStaticAPIPrefix, relFilePath, string(content))
			mdInfos = append(mdInfos, mdInfo{
				content:     convertByte,
				relFilePath: relFilePath,
			})
		}
		return nil
	}); err != nil {
		return err
	}

	// 1. 构建搜索引擎
	searcher, err := newDocMenuSearcher(mdInfos)
	if err != nil {
		return fmt.Errorf("init search engin err: %v", err)
	}

	// 2. 构造menus、contents
	var menus []*response.DocMenu
	contents := make(map[string]string)
	for _, mdInfo := range mdInfos {
		contents[mdInfo.relFilePath] = mdInfo.content
		addDocMenusMdFile(&menus, mdInfo.relFilePath, mdInfo)
	}

	// 3. 刷新索引
	// 3.1 menus排序
	sortDocMenus(&menus)
	// 3.2 重新生成menus index
	for i, menu := range menus {
		refreshDocMenuIndex(menu, fmt.Sprintf("doc%d", i+1))
	}

	_docCenter = &docCenter{
		menus:    menus,
		contents: contents,
		searcher: searcher,
	}
	return nil
}

func GetDocCenterMenu(ctx *gin.Context) []*response.DocMenu {
	return _docCenter.menus
}

func SearchDocCenter(ctx *gin.Context, content string) ([]response.DocSearchResp, error) {
	results := _docCenter.searcher.SearchDoc(types.SearchReq{Text: content})
	var searchResps []response.DocSearchResp
	for _, doc := range results.Docs {
		title := strings.TrimSuffix(filepath.Base(doc.DocId), filepath.Ext(filepath.Base(doc.DocId)))
		snippet, err := util.Md2html([]byte(getMarkdownSnippet(doc.Content, content, docCenterSnippetLen)))
		if err != nil {
			log.Errorf("doc center %v md2html error", doc.DocId)
			continue // 跳过当前doc不做处理
		}
		searchUrl, err := url.JoinPath(config.Cfg().Server.WebBaseUrl, config.Cfg().DocCenter.FrontendPrefix, url.PathEscape(doc.DocId))
		if err != nil {
			log.Errorf("doc center %v to search url err: %v", doc.DocId, err)
			continue
		}
		searchResp := response.DocSearchResp{
			Title: title,
			ContentList: []response.DocSearchContent{
				{
					Title:   title,
					Content: snippet,
					Url:     searchUrl,
				},
			},
		}
		searchResps = append(searchResps, searchResp)
	}
	return searchResps, nil
}

func GetDocCenterMarkdown(ctx *gin.Context, relFilePath string) (string, error) {
	relFilePath, err := url.QueryUnescape(relFilePath)
	if err != nil {
		return "", grpc_util.ErrorStatusWithKey(err_code.Code_BFFGeneral, "bff_doc_center_file_unescape", err.Error())
	}
	// check fileName
	if !strings.HasSuffix(relFilePath, ".md") {
		return "", grpc_util.ErrorStatusWithKey(err_code.Code_BFFGeneral, "bff_doc_center_file_md", relFilePath)
	}
	return _docCenter.contents[relFilePath], nil
}

// --- doc-center convert raw markdown ---

// 将markdown文本中图片引用 ![](xxxxx )与链接引用[](xxxxx )里的 xxxxx 处理为前端可访问的地址
//
//nolint:staticcheck
func convertMarkdown(apiPrefix, refFilePath, mdContent string) string {
	convertHttp := mdLinkRegex.ReplaceAllStringFunc(mdContent, func(mdLabel string) string {
		for _, httpRelPaths := range mdParenthesisRefRegex.FindAllStringSubmatch(mdLabel, -1) {
			if len(httpRelPaths) <= 1 {
				return mdLabel
			}
			txt := mdBracketRegex.FindString(mdLabel)
			return txt + "(" + url.PathEscape(path.Join(refFilePath, "../", httpRelPaths[1])) + ")"
		}
		return mdLabel
	})
	return mdImageRegex.ReplaceAllStringFunc(convertHttp, func(imageLabel string) string {
		// imageLabel是匹配到的图片格式，例如 ![](../assets/append.png)
		for _, imageRelPaths := range mdParenthesisRefRegex.FindAllStringSubmatch(imageLabel, -1) {
			// 从imageLabel中继续匹配，例如imageRelPaths[0] 是 ![](../assets/append.png)，imageRelPaths[1]是 ../assets/append.png
			if len(imageRelPaths) <= 1 {
				return imageLabel
			}
			// 重新生成图片引用，将 ../assets/append.png 处理为 user/api/v1/static/manual/assets/append.png
			// 例如refFilePath是workflow/StartNode.md
			// 1. "workflow/StartNode.md" + "../" + "../assets/append.png" => assets/append.png
			// 2. "../../../user/api/v1/static/manual" + "assets/append.png" => ../../../user/api/v1/static/manual/assets/append.png
			// 3. 对路径中的非数字字母等做转义，再将 %2F 转回 /
			return "![](" + strings.ReplaceAll(url.PathEscape(path.Join(apiPrefix, path.Join(refFilePath, "../", imageRelPaths[1]))), "%2F", "/") + ")"
		}
		return imageLabel
	})
}

// --- doc-center search engine ---

func newDocMenuSearcher(mdInfos []mdInfo) (*riot.Engine, error) {
	engine := &riot.Engine{}
	engine.Init(types.EngineOpts{})
	// 创建索引
	for _, mdInfo := range mdInfos {
		engine.Index(mdInfo.relFilePath, types.DocData{
			Content: string(mdInfo.content),
		})
	}
	// 刷新索引
	engine.Flush()
	return engine, nil
}

func getMarkdownSnippet(content, keyword string, snippetLen int) string {
	//string就是只读的采用utf8编码的字节切片(slice) 因此用len函数获取到的长度并不是字符个数，而是字节个数。
	//rune是int32的别名，代表字符的Unicode编码，采用4个字节存储，将string转成rune就意味着任何一个字符都用4个字节来存储其unicode值，
	//这样每次遍历的时候返回的就是unicode值，而不再是字节。
	runes := []rune(content)
	keyRunes := []rune(keyword)
	index := strings.Index(content, keyword)
	if index == -1 {
		if len(runes) < snippetLen {
			return string(runes)
		} else {
			return string(runes[0:snippetLen])
		}
	}
	runeIndex := len([]rune(content[:index]))
	start := runeIndex - snippetLen
	if start < 0 {
		start = 0
	}
	end := runeIndex + len(keyRunes) + snippetLen
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// --- doc-center add markdown file info to menus ---

func addDocMenusMdFile(menus *[]*response.DocMenu, rest string, mdInfo mdInfo) {
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 {
		return
	}
	var menu *response.DocMenu
	for _, curr := range *menus {
		if curr.Name == parts[0] {
			menu = curr
			break
		}
	}
	if menu == nil {
		menu = &response.DocMenu{
			Name: parts[0],
		}
		if len(parts) == 1 { // 非目录，是md文件
			menu.Name = strings.TrimSuffix(menu.Name, ".md")
			menu.PathRaw = mdInfo.relFilePath
			menu.Path = url.PathEscape(mdInfo.relFilePath) // 前端要求做path转义
			menu.SetContent(mdInfo.content)
		}
		*menus = append(*menus, menu)
	}
	if len(parts) > 1 {
		addDocMenusMdFile(&(menu.Children), parts[1], mdInfo)
	}
}

// --- doc-center sort menus ---

func sortDocMenus(menus *[]*response.DocMenu) {
	sort.Slice(*menus, func(i, j int) bool {
		return orderDocNum((*menus)[i].Name, (*menus)[j].Name)
	})
	for _, menu := range *menus {
		if len(menu.Children) > 0 {
			sortDocMenus(&menu.Children)
		}
	}
}

// 实现自然排序（数字优先）
func orderDocNum(s1, s2 string) bool {
	numParts1, isNum1 := extractDocNum(s1)
	numParts2, isNum2 := extractDocNum(s2)
	if isNum1 && isNum2 {
		return numParts1 < numParts2
	} else if isNum1 {
		// 如果一个是数字，一个是非数字，数字部分排在前
		return true
	} else if isNum2 {
		return false
	}
	return s1 < s2
}

// extractNum 将字符串按数字和非数字部分分割
func extractDocNum(s string) (int, bool) {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			result.WriteRune(r)
		} else {
			break
		}
	}
	num, err := strconv.Atoi(result.String())
	if err != nil {
		return 0, false
	}
	return num, true
}

// --- doc-center refresh index ---

func refreshDocMenuIndex(menu *response.DocMenu, index string) {
	menu.Index = index
	for i, child := range menu.Children {
		refreshDocMenuIndex(child, fmt.Sprintf("%s-%d", index, i+1))
	}
}
