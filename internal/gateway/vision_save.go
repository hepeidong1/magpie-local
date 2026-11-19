package gateway

// 拒图学习的落盘与回载（image-blind.json）：模型拒图的"看不见"记录
// 原本只存内存（Server.blind，refuseSight），重启即忘——models.dev 对
// 小中转/免费 API 根本没有 images 数据（本机核对：组里 25/30 个供应商
// 不在目录里，在目录里的条目 images 也是空），"清单没说"的条目每次
// 重启后都要再白跑一趟（把整段对话连同 base64 图上传上去听一句"不支持
// 图片"）才重新学会"我不收图"。拒图很少发生，落盘每次同步写即可；
// 启动时装回。

import (
	"encoding/json"
	"log"
	"os"
	"slices"
	"sync"

	"github.com/yetone/magpie/internal/appdir"
)

// blindMu 是 image-blind.json 文件层的小锁：blind 本身是 sync.Map，
// 进程内自洽；多个 Server 共存（测试）时靠它串行化文件写。
var blindMu sync.Mutex

func imageBlindPath() string { return appdir.Join(appdir.Config(), "image-blind.json") }

// saveBlind 把当前 blind 集合整体写到 image-blind.json（tmp+rename，
// 写失败只记日志——丢一条学习记录，下次拒图再学就是）。
func (s *Server) saveBlind() {
	blindMu.Lock()
	defer blindMu.Unlock()
	keys := []string{}
	s.blind.Range(func(k, _ any) bool {
		keys = append(keys, k.(string))
		return true
	})
	slices.Sort(keys)
	b, err := json.Marshal(keys)
	if err != nil {
		return
	}
	tmp := imageBlindPath() + ".tmp"
	if err := os.MkdirAll(appdir.Config(), 0o700); err != nil {
		log.Printf("magpie: could not save the learned image blindness: %v", err)
		return
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("magpie: could not save the learned image blindness: %v", err)
		return
	}
	if err := os.Rename(tmp, imageBlindPath()); err != nil {
		log.Printf("magpie: could not save the learned image blindness: %v", err)
	}
}

// loadBlind 回载上一次学到的拒图记录（Serve 启动时装回）。
func (s *Server) loadBlind() {
	b, err := os.ReadFile(imageBlindPath())
	if err != nil {
		return
	}
	var keys []string
	if err := json.Unmarshal(b, &keys); err != nil {
		log.Printf("magpie: could not read the learned image blindness: %v", err)
		return
	}
	for _, k := range keys {
		s.blind.Store(k, true)
	}
}
