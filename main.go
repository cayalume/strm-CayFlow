package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "net/http"
    "net/http/httputil"
    "net/url"
    "os"
    "log"
    "os/exec"
    "regexp"
    "strings"
    "sync"
    "time"

    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
    "github.com/gin-gonic/gin"
    "github.com/patrickmn/go-cache"

    "bot-emby/config"
    "bot-emby/controllers"
)


func GinAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        session := sessions.Default(c)
        auth := session.Get("authenticated")
        if auth != nil && auth.(bool) {
            c.Next()
            return
        }
        if c.GetHeader("Accept") == "application/json" || c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "ok":  false,
                "msg": "未授权，请重新登录",
            })
        } else {
            c.Redirect(http.StatusFound, "/login")
            c.Abort()
        }
    }
}

func loginHandler(c *gin.Context) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数错误"})
        return
    }
    cfg := config.GetInstance()
    expectedUser, expectedPass := cfg.GetAdminCredentials()
    if expectedUser == "" {
        expectedUser = "admin"
        expectedPass = "admin"
    }
    if req.Username == expectedUser && req.Password == expectedPass {
        session := sessions.Default(c)
        session.Set("authenticated", true)
        session.Save()
        c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "登录成功"})
    } else {
        c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "用户名或密码错误"})
    }
}

func logoutHandler(c *gin.Context) {
    session := sessions.Default(c)
    session.Delete("authenticated")
    session.Save()
    c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "已退出登录"})
}

func notificationHandler(w http.ResponseWriter, r *http.Request) {
    cfg := config.GetInstance()
    tpl, err := template.ParseFiles("templates/notification.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tpl.Execute(w, map[string]interface{}{
        "tg_channels": cfg.GetTgChannels(),
    })
}

func getEmbyConfigHandler(w http.ResponseWriter, r *http.Request) {
    cfg := config.GetInstance()
    embyUrl, embyApiKey := cfg.GetEmbyConfig()
    embyUserId := cfg.GetEmbyUserId()
    json.NewEncoder(w).Encode(map[string]interface{}{
        "ok":            true,
        "emby_url":      embyUrl,
        "emby_api_key":  embyApiKey,
        "emby_user_id":  embyUserId,
    })
}

func updateEmbyConfigHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Url    string `json:"url"`
        Key    string `json:"api_key"`
        UserId string `json:"user_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    cfg := config.GetInstance()
    cfg.SetEmbyConfig(req.Url, req.Key)
    if req.UserId != "" {
        cfg.SetEmbyUserId(req.UserId)
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "Emby 配置已更新"})
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
    cfg := config.GetInstance()
    json.NewEncoder(w).Encode(map[string]interface{}{
        "tg_channels": cfg.GetTgChannels(),
    })
}

func getSystemConfigHandler(w http.ResponseWriter, r *http.Request) {
    cfg := config.GetInstance()
    username, _ := cfg.GetAdminCredentials()
    json.NewEncoder(w).Encode(map[string]interface{}{
        "ok":       true,
        "username": username,
    })
}

func updateSystemCredentialsHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Username    string `json:"username"`
        Password    string `json:"password"`
        ConfirmPass string `json:"confirm_password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if req.Username == "" || req.Password == "" {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "用户名和密码不能为空"})
        return
    }
    if req.Password != req.ConfirmPass {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "两次输入的密码不一致"})
        return
    }
    cfg := config.GetInstance()
    cfg.SetAdminCredentials(req.Username, req.Password)
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "用户名密码已更新，请重新登录"})
}

func createTgChannelHandler(w http.ResponseWriter, r *http.Request) {
    var channel config.TgChannel
    if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    botToken := channel.BotToken
    if botToken == "" {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "ok":  false,
            "msg": "Bot Token 不能为空",
        })
        return
    }
    baseURL := channel.ApiURL
    if baseURL == "" {
        baseURL = "https://api.telegram.org"
    }
    client := &http.Client{Timeout: 10 * time.Second}
    apiURL := fmt.Sprintf("%s/bot%s/getMe", baseURL, botToken)
    resp, err := client.Get(apiURL)
    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "ok":  false,
            "msg": fmt.Sprintf("网络错误: %v", err),
        })
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "ok":  false,
            "msg": fmt.Sprintf("Telegram API 返回错误 (HTTP %d)", resp.StatusCode),
        })
        return
    }
    var tgResp struct {
        Ok     bool   `json:"ok"`
        Result struct {
            Username string `json:"username"`
        } `json:"result"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil || !tgResp.Ok {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "ok":  false,
            "msg": "Bot Token 无效",
        })
        return
    }
    channel.ID = fmt.Sprintf("%d", time.Now().UnixNano())
    channel.Enable = true
    if channel.Rules == nil {
        channel.Rules = make(map[string]bool)
    }
    config.GetInstance().AddTgChannel(channel)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "ok":   true,
        "msg":  fmt.Sprintf("创建成功 (Bot: @%s)", tgResp.Result.Username),
        "data": channel,
    })
}

// ---------- 版本更新检查 ----------
type dockerHubTagsResponse struct {
    Results []struct {
        Name string `json:"name"`
    } `json:"results"`
}

func getLatestVersionFromDockerHub(repo string) (string, error) {
    url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=10&ordering=last_updated", repo)
    client := http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var data dockerHubTagsResponse
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return "", err
    }

    var versions []string
    tagPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
    for _, tag := range data.Results {
        if tagPattern.MatchString(tag.Name) {
            versions = append(versions, tag.Name)
        }
    }
    if len(versions) == 0 {
        return "", fmt.Errorf("no semantic version tags found")
    }

    sortVersions := func(v1, v2 string) bool {
        normalize := func(v string) []int {
            v = strings.TrimPrefix(v, "v")
            parts := strings.Split(v, ".")
            nums := make([]int, 3)
            for i, p := range parts {
                if i < 3 {
                    fmt.Sscanf(p, "%d", &nums[i])
                }
            }
            return nums
        }
        n1 := normalize(v1)
        n2 := normalize(v2)
        for i := 0; i < 3; i++ {
            if n1[i] != n2[i] {
                return n1[i] > n2[i]
            }
        }
        return false
    }
    latest := versions[0]
    for _, v := range versions[1:] {
        if sortVersions(v, latest) {
            latest = v
        }
    }
    if !strings.HasPrefix(latest, "v") {
        latest = "v" + latest
    }
    return latest, nil
}

func checkUpdateHandler(c *gin.Context) {
    repo := c.Query("repo")
    currentVersion := c.Query("current")
    if repo == "" || currentVersion == "" {
        c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "missing repo or current version"})
        return
    }

    latestVersion, err := getLatestVersionFromDockerHub(repo)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": err.Error()})
        return
    }

    hasUpdate := (latestVersion != currentVersion)
    c.JSON(http.StatusOK, gin.H{
        "ok":             true,
        "hasUpdate":      hasUpdate,
        "latestVersion":  latestVersion,
    })
}

func testTgChannelHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    var req struct {
        ID string `json:"id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "请求参数错误"})
        return
    }
    cfg := config.GetInstance()
    ch := cfg.GetTgChannelByID(req.ID)
    if ch == nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "渠道不存在"})
        return
    }
    if !ch.Enable {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "渠道未启用"})
        return
    }
    baseURL := ch.ApiURL
    if baseURL == "" {
        baseURL = "https://api.telegram.org"
    }
    testMsg := "🔔 这是一条来自 Emby Bot 的测试消息。"
    apiURL := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, ch.BotToken)
    payload, _ := json.Marshal(map[string]string{
        "chat_id":    ch.ChatID,
        "text":       testMsg,
        "parse_mode": "HTML",
    })
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": fmt.Sprintf("网络错误: %v", err)})
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": fmt.Sprintf("Telegram 返回错误: %s", string(body))})
        return
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "测试消息发送成功"})
}

func updateTgChannelHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID       string `json:"id"`
        Name     string `json:"name"`
        BotToken string `json:"bot_token"`
        ChatID   string `json:"chat_id"`
        ApiURL   string `json:"api_url"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    cfg := config.GetInstance()
    baseURL := req.ApiURL
    if baseURL == "" {
        baseURL = "https://api.telegram.org"
    }
    client := &http.Client{Timeout: 10 * time.Second}
    apiURL := fmt.Sprintf("%s/bot%s/getMe", baseURL, req.BotToken)
    resp, err := client.Get(apiURL)
    if err != nil || resp.StatusCode != 200 {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "Bot Token 无效或网络错误"})
        return
    }
    defer resp.Body.Close()
    cfg.UpdateTgChannel(req.ID, req.Name, req.BotToken, req.ChatID, req.ApiURL)
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "更新成功"})
}

func deleteTgChannelHandler(w http.ResponseWriter, r *http.Request) {
    var req struct{ ID string }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    config.GetInstance().DeleteTgChannel(req.ID)
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "删除成功"})
}

func toggleTgChannelHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID     string
        Enable bool
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    config.GetInstance().ToggleTgChannel(req.ID, req.Enable)
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "状态已更新"})
}

func getChannelRulesHandler(w http.ResponseWriter, r *http.Request) {
    channelID := r.URL.Query().Get("channel_id")
    if channelID == "" {
        http.Error(w, "missing channel_id", http.StatusBadRequest)
        return
    }
    cfg := config.GetInstance()
    ch := cfg.GetTgChannelByID(channelID)
    if ch == nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "msg": "渠道不存在"})
        return
    }
    if ch.Rules == nil {
        ch.Rules = make(map[string]bool)
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "rules": ch.Rules})
}

func saveChannelRulesHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ChannelID string          `json:"channel_id"`
        Rules     map[string]bool `json:"rules"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    config.GetInstance().SetChannelRules(req.ChannelID, req.Rules)
    json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "msg": "规则已保存"})
}

func main() {
    r := gin.Default()

    // ========== 初始化定时调度器 ==========
    controllers.InitScheduler()
    controllers.InitScrapeScheduler()
    // 启动 Telegram Bot
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("[CRITICAL] Telegram Bot 启动 Panic: %v", r)
            }
        }()
        controllers.StartTelegramBot()
    }()


    secret := []byte("emby-bot-secret-key-2024")
    store := cookie.NewStore(secret)
    store.Options(sessions.Options{
        Path:     "/",
        MaxAge:   86400 * 7,
        HttpOnly: true,
        Secure:   false,
        SameSite: http.SameSiteLaxMode,
    })
    r.Use(sessions.Sessions("emby_bot_session", store))
    r.Static("/posters", "/app/poster/output")
    r.LoadHTMLGlob("templates/*.html")
    r.Static("/icon", "./icon")
    r.Static("/assets", "./assets")
    r.GET("/login", func(c *gin.Context) {
        c.HTML(http.StatusOK, "login.html", nil)
    })
    r.POST("/api/login", loginHandler)
    r.POST("/api/logout", logoutHandler)

    // ========== 需要认证的 API 分组 ==========
    authGroup := r.Group("/")
    authGroup.Use(GinAuth())
    {
        authGroup.GET("/", gin.WrapF(notificationHandler))
        authGroup.GET("/notification", gin.WrapF(notificationHandler))

        api := authGroup.Group("/api")
        {
            api.GET("/config", gin.WrapF(getConfigHandler))
            api.POST("/notification/tg/create", gin.WrapF(createTgChannelHandler))
            api.POST("/notification/tg/update", gin.WrapF(updateTgChannelHandler))
            api.POST("/notification/tg/delete", gin.WrapF(deleteTgChannelHandler))
            api.POST("/notification/tg/toggle", gin.WrapF(toggleTgChannelHandler))
            api.GET("/notification/tg/rules", gin.WrapF(getChannelRulesHandler))
            api.POST("/notification/tg/rules", gin.WrapF(saveChannelRulesHandler))
            api.POST("/notification/tg/test", gin.WrapF(testTgChannelHandler))
            api.GET("/emby/config", gin.WrapF(getEmbyConfigHandler))
            api.POST("/emby/config", gin.WrapF(updateEmbyConfigHandler))
            api.GET("/system/config", gin.WrapF(getSystemConfigHandler))
            api.POST("/system/credentials", gin.WrapF(updateSystemCredentialsHandler))

            api.GET("/strm/libs", controllers.GetStrmLibsHandler)
            api.POST("/strm/libs", controllers.AddStrmLibHandler)
            api.PUT("/strm/libs/:key", controllers.UpdateStrmLibHandler)
            api.DELETE("/strm/libs/:key", controllers.DeleteStrmLibHandler)
            api.POST("/strm/sync/:key", controllers.SyncStrmLibHandler)
            api.POST("/strm/stop/:key", controllers.StopStrmLibHandler)
            api.GET("/strm/log/:key", controllers.GetStrmLogHandler)
            api.GET("/browse", controllers.BrowseDirectoryGin)

            api.GET("/115/accounts", controllers.GetAccountsHandler)
            api.POST("/115/accounts", controllers.AddAccountHandler)
            api.PUT("/115/accounts/:id", controllers.UpdateAccountHandler)
            api.DELETE("/115/accounts/:id", controllers.DeleteAccountHandler)
            api.POST("/115/accounts/:id/refresh", controllers.RefreshAccountHandler)

            api.GET("/strm/settings", controllers.GetStrmSettingsHandler)
            api.POST("/strm/settings", controllers.SaveStrmSettingsHandler)
            api.GET("/poster/config", controllers.GetPosterConfigHandler)
            api.POST("/poster/config", controllers.SavePosterConfigHandler)
            api.POST("/poster/generate", controllers.GeneratePosterHandler)
            api.GET("/poster/log", controllers.GetPosterLogHandler)
            api.POST("/poster/log/clear", controllers.ClearPosterLogHandler)
            api.GET("/poster/libraries", controllers.GetLibrariesHandler)
            api.POST("/poster/generate/:libraryId", controllers.GenerateSinglePosterHandler)
            api.GET("/poster/library/:libraryId/config", controllers.GetLibraryConfigHandler)
            api.POST("/poster/library/:libraryId/config", controllers.SaveLibraryConfigHandler)
            api.GET("/poster/library/:libraryId/log", controllers.GetLibraryLogHandler)
            api.GET("/emby/play_duration", controllers.GetPlayDurationHandler)
            api.POST("/poster/library/:libraryId/log/clear", controllers.ClearLibraryLogHandler)
            api.GET("/proxy302/config", getProxy302ConfigHandler)
            api.POST("/proxy302/config", updateProxy302ConfigHandler)
            api.GET("/tmdb/config", controllers.GetTmdbConfigHandler)
            api.POST("/tmdb/config", controllers.SaveTmdbConfigHandler)
            api.GET("/scraper/tasks", controllers.GetScrapeTasksHandler)
            api.POST("/scraper/task", controllers.SaveScrapeTaskHandler)
            api.DELETE("/scraper/task/:id", controllers.DeleteScrapeTaskHandler)
            api.POST("/scraper/start", controllers.StartScrapeTaskHandler)
            api.POST("/scraper/stop", controllers.StopScrapeTaskHandler)
            api.GET("/scraper/log/:taskID", controllers.GetScrapeLogHandler)
            api.GET("/check-update", checkUpdateHandler)
            api.GET("/emby/image/:id", controllers.ProxyEmbyImage)
            api.GET("/emby/libraries", controllers.GetEmbyLibraries)
            api.GET("/emby/missing", controllers.GetMissingEpisodesReport)
            api.GET("/emby/stats", controllers.GetEmbyStats)
            api.GET("/emby/libraries/stats", controllers.GetLibraryContentStats)
            api.GET("/emby/recently_added", controllers.GetRecentlyAdded)
            api.GET("/115/browse", controllers.Get115DirTreeHandler)
        }
    }

    r.POST("/emby/webhook", controllers.Webhook)

    fmt.Println("✅ 服务启动成功，端口：8092")
    r.Run("0.0.0.0:8092")
}