// C:\Users\wasab\OneDrive\デスクトップ\WASABI\config\config.go

package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Config はアプリケーションの設定情報を保持する構造体です。
// `config.json`ファイルにこの構造体の内容が保存されます。
type Config struct {
	EmednetUserID   string `json:"emednetUserId"`
	EmednetPassword string `json:"emednetPassword"`
	EdeUserID       string `json:"edeUserId"`
	EdePassword     string `json:"edePassword"`
	UsageFolderPath string `json:"usageFolderPath"`
	// ▼▼▼【ここから修正】▼▼▼
	// 2つの日付フィールドを削除し、集計日数を保持するフィールドを1つ追加
	CalculationPeriodDays int `json:"calculationPeriodDays"`
	// ▲▲▲【修正ここまで】▲▲▲
	// ▼▼▼【ここに追加】▼▼▼
	EdgePath string `json:"edgePath"` // Edgeの実行可能ファイルパス
	// ▲▲▲【追加ここまで】▲▲▲
}

var (
	// cfg はアプリケーション全体で共有される設定情報を保持するグローバル変数です。
	cfg Config
	// mu は設定情報への同時アクセスを防ぎ、データの競合を避けるためのロックです。
	mu sync.RWMutex
)

// configFilePath は設定ファイルのパスを定義する定数です。
const configFilePath = "./config.json"

/**
 * @brief config.json ファイルから設定を読み込み、メモリにキャッシュします。
 * @return Config 読み込まれた設定情報
 * @return error ファイルの読み込みや解析中にエラーが発生した場合
 * @details
 * ファイルが存在しない場合は、空の設定情報とnilエラーを返します。
 * 読み込み中は読み取りロックをかけ、スレッドセーフを保証します。
 */
func LoadConfig() (Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	file, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// ファイルが存在しないのは初回起動時などの正常なケースなのでエラーとはしない
			return Config{
				// ▼▼▼【修正】日数のデフォルト値を設定 ▼▼▼
				CalculationPeriodDays: 90,
			}, nil
		}
		return Config{}, err
	}

	var tempCfg Config
	if err := json.Unmarshal(file, &tempCfg); err != nil {
		return Config{}, err
	}
	cfg = tempCfg
	return cfg, nil
}

/**
 * @brief 新しい設定情報を config.json ファイルに保存します。
 * @param newCfg 保存する新しい設定情報
 * @return error ファイルの書き込み中にエラーが発生した場合
 * @details
 * 書き込み中は書き込みロックをかけ、スレッドセーフを保証します。
 * 保存が成功すると、メモリ上のグローバルな設定情報も更新されます。
 */
func SaveConfig(newCfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	file, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(configFilePath, file, 0644); err != nil {
		return err
	}
	cfg = newCfg
	return nil
}

/**
 * @brief メモリにキャッシュされている現在の設定情報を取得します。
 * @return Config 現在の設定情報
 * @details
 * 読み取り中は読み取りロックをかけ、スレッドセーフを保証します。
 */
func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}
