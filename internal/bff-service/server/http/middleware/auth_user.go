package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/bff-service/service"
	gin_util "github.com/UnicomAI/wanwu/pkg/gin-util"
	"github.com/UnicomAI/wanwu/pkg/gin-util/route"
	jwt_util "github.com/UnicomAI/wanwu/pkg/jwt-util"
	"github.com/gin-gonic/gin"
)

func CheckUserEnable(ctx *gin.Context) {
	httpStatus := http.StatusForbidden
	// userID
	userID, err := getUserID(ctx)
	if err != nil {
		gin_util.ResponseErrCodeKeyWithStatus(ctx, httpStatus, err_code.Code_BFFAuth, "", err.Error())
		ctx.Abort()
		return
	}
	ctx.Set(gin_util.USER_ID, userID)
	// genTokenTS
	genTokenTS, err := getGenTokenTS(ctx)
	if err != nil {
		gin_util.ResponseErrCodeKeyWithStatus(ctx, httpStatus, err_code.Code_BFFAuth, "", err.Error())
		ctx.Abort()
		return
	}
	// check
	if resp, err := service.CheckUserEnable(ctx, userID, genTokenTS); err != nil {
		gin_util.ResponseErrWithStatus(ctx, httpStatus, err)
		ctx.Abort()
		return
	} else {
		ctx.Set(gin_util.X_LANGUAGE, resp.Language)
	}
}

func CheckUserPerm(ctx *gin.Context) {
	httpStatus := http.StatusForbidden
	// userID
	userID, err := getUserID(ctx)
	if err != nil {
		gin_util.ResponseErrCodeKeyWithStatus(ctx, httpStatus, err_code.Code_BFFAuth, "", err.Error())
		ctx.Abort()
		return
	}
	ctx.Set(gin_util.USER_ID, userID)
	// genTokenTS
	genTokenTS, err := getGenTokenTS(ctx)
	if err != nil {
		gin_util.ResponseErrCodeKeyWithStatus(ctx, httpStatus, err_code.Code_BFFAuth, "", err.Error())
		ctx.Abort()
		return
	}
	// orgID
	orgID := getOrgID(ctx)
	// tags
	tags, ok := route.GetTags(ctx.FullPath(), ctx.Request.Method)
	if !ok {
		gin_util.ResponseErrCodeKeyWithStatus(ctx, httpStatus, err_code.Code_BFFGeneral, "", fmt.Sprintf("auth path [%v]%v not found", ctx.Request.Method, ctx.FullPath()))
		ctx.Abort()
		return
	}
	// check
	if resp, err := service.CheckUserPerm(ctx, userID, genTokenTS, orgID, tags); err != nil {
		gin_util.ResponseErrWithStatus(ctx, httpStatus, err)
		ctx.Abort()
		return
	} else {
		ctx.Set(gin_util.IS_ADMIN, resp.IsAdmin)
		ctx.Set(gin_util.IS_SYSTEM, resp.IsSystem)
	}
}

// --- internal ---

func getUserID(ctx *gin.Context) (string, error) {
	claims, ok := ctx.Get(gin_util.CLAIMS)
	if !ok {
		return "", errors.New("jwt claims empty")
	}
	return claims.(*jwt_util.CustomClaims).UserID, nil
}

func getOrgID(ctx *gin.Context) string {
	return ctx.GetHeader(gin_util.X_ORG_ID)
}

func getGenTokenTS(ctx *gin.Context) (string, error) {
	claims, ok := ctx.Get(gin_util.CLAIMS)
	if !ok {
		return "", errors.New("jwt claims empty")
	}
	return strconv.Itoa(int(claims.(*jwt_util.CustomClaims).ExpiresAt * 1000)), nil
}
