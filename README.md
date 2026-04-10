# Magi Link

ヘックスグリッド × スペルチェイン ローグライトRPG

## 概要

Magi（魔法使い）はLink（連鎖）することで魔法の真価を発揮する。本来は複数人で組むLinkを、一人で完結させられる異端のMagiとして、残響（Resonance）が巣食う迷宮に潜る。

## 技術スタック

- **言語**: Go
- **描画**: [Ebitengine](https://ebitengine.org/)
- **ターゲット**: WebAssembly (WASM) + デスクトップ

## プロジェクト構成

```
magi_link/
├── cmd/game/        # エントリポイント
├── internal/
│   ├── game/        # ゲームループ・状態管理
│   ├── hex/         # ヘックスグリッド（Cube座標）
│   ├── spell/       # スペルチェイン・スペルブック
│   ├── passive/     # パッシブスキル
│   ├── entity/      # ユニット（プレイヤー・敵）
│   ├── terrain/     # 地形・相互作用
│   └── ui/          # UI・入力処理
└── assets/          # 画像・音声
```

## ビルド・実行

```bash
# デスクトップ実行
go run ./cmd/game

# WASM ビルド
GOOS=js GOARCH=wasm go build -o web/main.wasm ./cmd/game
```

## ゲームデザイン仕様

[magi_link_design_spec.md](./magi_link_design_spec.md) を参照。
