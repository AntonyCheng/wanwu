package gin_util

import (
	"encoding/json"
	"fmt"
	"net/http"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/bff-service/config"
	"github.com/UnicomAI/wanwu/internal/bff-service/pkg/i18n"
	"github.com/UnicomAI/wanwu/internal/bff-service/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_v *validator.Validate
)

// --- gin validator ---

type ginValidator struct{}

func (gv *ginValidator) Engine() interface{} {
	return _v
}

func (gv *ginValidator) ValidateStruct(obj interface{}) error {
	return util.Validate(obj)
}

func InitValidator() error {
	if err := util.InitValidator(); err != nil {
		return err
	}
	binding.Validator = &ginValidator{}
	return nil
}

// --- bind ---

type iChecker interface {
	Check() error
}

func Bind(ctx *gin.Context, param iChecker) bool {
	if err := ctx.ShouldBindBodyWith(param, binding.JSON); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	if err := param.Check(); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	return true
}

func BindUri(ctx *gin.Context, param iChecker) bool {
	if err := ctx.ShouldBindUri(param); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	if err := param.Check(); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	return true
}

func BindForm(ctx *gin.Context, param iChecker) bool {
	if err := ctx.ShouldBind(param); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	if err := param.Check(); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	return true
}

func BindQuery(ctx *gin.Context, param iChecker) bool {
	if err := ctx.ShouldBindQuery(param); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	if err := param.Check(); err != nil {
		ResponseErrCodeKey(ctx, err_code.Code_BFFInvalidArg, "", err.Error())
		return false
	}
	return true
}

// --- response ---

// Response 返回200与data信息，或者400与err信息，err有i18n
func Response(ctx *gin.Context, data interface{}, err error) {
	if err != nil {
		ResponseErr(ctx, err)
		return
	}
	ResponseOKWithData(ctx, data)
}

// ResponseOK 返回200
func ResponseOK(ctx *gin.Context) {
	ResponseDetail(ctx, http.StatusOK, codes.OK, nil, "")
}

// ResponseOKWithData 返回200与data信息
func ResponseOKWithData(ctx *gin.Context, data interface{}) {
	ResponseDetail(ctx, http.StatusOK, codes.OK, data, "")
}

// ResponseDetail 返回400与err信息，err有i18n
func ResponseErr(ctx *gin.Context, err error) {
	ResponseErrWithStatus(ctx, http.StatusBadRequest, err)
}

// ResponseDetail 返回httpStatus与err信息，err有i18n
func ResponseErrWithStatus(ctx *gin.Context, httpStatus int, err error) {
	st, ok := status.FromError(err)
	if !ok {
		ResponseDetail(ctx, httpStatus, codes.Code(err_code.Code_BFFGeneral), nil, fmt.Sprintf("[i18n] %v", err))
		return
	}
	for _, detail := range st.Details() {
		switch detail := detail.(type) {
		case *err_code.Status:
			ResponseDetail(ctx, httpStatus, st.Code(), nil, I18nCodeOrKey(ctx, err_code.Code(st.Code()), detail.TextKey, detail.Args...))
			return
		}
	}
	ResponseDetail(ctx, httpStatus, st.Code(), nil, fmt.Sprintf("[i18n] %v", st.Message()))
}

// ResponseErrCodeKey 返回400/code与错误信息，code/key有i18n
func ResponseErrCodeKey(ctx *gin.Context, code err_code.Code, textKey string, args ...string) {
	ResponseDetail(ctx, http.StatusBadRequest, codes.Code(code), nil, I18nCodeOrKey(ctx, code, textKey, args...))
}

// ResponseErrCodeKey 返回httpStatus/code与错误信息，code/key有i18n
func ResponseErrCodeKeyWithStatus(ctx *gin.Context, httpStatus int, code err_code.Code, textKey string, args ...string) {
	ResponseDetail(ctx, httpStatus, codes.Code(code), nil, I18nCodeOrKey(ctx, code, textKey, args...))
}

// ResponseDetail 直接返回httpStatus/code/data/msg，msg无i18n
func ResponseDetail(ctx *gin.Context, httpStatus int, code codes.Code, data interface{}, msg string) {
	resp := &response{
		Code: int64(code),
		Data: data,
		Msg:  msg,
	}
	b, _ := json.Marshal(resp)
	ctx.Set(config.STATUS, httpStatus)
	ctx.Set(config.RESULT, string(b))
	ctx.JSON(httpStatus, resp)
}

// response 与model/response中Response一致，后者只用于swagger生成
type response struct {
	Code int64       `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

// --- i18n ---

func I18nCode(ctx *gin.Context, code err_code.Code, args ...string) string {
	return I18nCodeOrKey(ctx, code, "", args...)
}

func I18nKey(ctx *gin.Context, key string, args ...string) string {
	return I18nCodeOrKey(ctx, 0, key, args...)
}

func I18nCodeOrKey(ctx *gin.Context, code err_code.Code, key string, args ...string) string {
	return i18n.ByCodeOrKey(getLanguage(ctx), code, key, args)
}

// --- internal ---

func getLanguage(ctx *gin.Context) i18n.Lang {
	// 1. 优先header的language
	language := ctx.GetHeader(config.X_LANGUAGE)
	// 2. 其次用户本设置的language
	if language == "" {
		language = ctx.GetString(config.X_LANGUAGE)
	}
	// 3. 再次系统默认的language
	if language == "" {
		language = config.Cfg().I18n.DefaultLang
	}
	return i18n.Lang(language)
}
