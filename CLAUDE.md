# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

本ファイルは Claude Code (claude.ai/code) が本リポジトリで作業する際の
ガイドラインを記述する。ゲームデザインの全体像は
[magi_link_design_spec.md](./magi_link_design_spec.md) を参照。

## Common commands

- `make run` / `go run ./cmd/game` — デスクトップ版を起動 (Ebitengine)
- `make build` — WASM ビルド (`web/main.wasm`) と `wasm_exec.js` の配置
- `go build ./...` — 通常ビルド確認
- `GOOS=js GOARCH=wasm go build ./...` — WASM ビルド確認 (ターゲット必須)
- `go test ./...` — 全テスト
- `go test ./internal/resolve -run TestExecuteLink` — パッケージ単位 / 単体テスト
- `go test ./internal/resolve -run TestExecuteLink/case_name -v` — testdata の 1 ケースのみ実行

## Project policy

### 既存実装・テストの扱い

**大きな設計変更を行うときは、既存の実装・テスト・テストデータを全部
消して書き直して良い。** 後方互換性や段階的移行よりも、新モデルの
一貫性・可読性を優先する方針。

- 古いテストや testdata が新モデルと食い違う場合は、遠慮なく削除して
  新モデル向けに書き直す。
- 「一応残しておく」「将来のために取っておく」は避ける。消すべきものは
  消す。
- この方針は小さな修正には適用しない（バグフィックスや局所的リファクタ
  では既存の構造を尊重する）。大規模な設計変更のときだけ解禁される。

### スペルシステムの設計原則

スペルは「バケツリレー構造」で解決される。1つの Link を実行するときは、
`LinkState = (Origins, Targets, Hexes, Flags)` を各スペルが順に変換する
パイプラインとして処理する。

- **Origins**: 次のアクション/モディファイアが発火する起点ヘックス
- **Targets**: 蓄積されたヒット対象（削除されない）
- **Hexes**: 影響を受けた全ヘックス（描画用）
- **Flags**: 次のステップを修飾する一時フラグ（pierce/bounce 等）

アクション（ダメージ、ヒール、状態付与）は Phase 2 として分離されており、
resolve パッケージは Targets を計算するのみ。

## Architecture overview

- `internal/spell/` — スペル定義の読み込み (spells.toml)、コスト計算、
  チェイン管理
- `internal/resolve/` — Link の解決パイプライン。`ExecuteLink(input, bf)`
  が LinkState を順に変換し、最終的な Targets/Hexes を返す
- `internal/hex/` — ヘックス座標系・グリッドユーティリティ
- `internal/entity/` — ユニット・ステータス
- `internal/terrain/` — 地形マップ・属性
- `internal/game/` — ebiten ゲームループ、バトル画面、入力処理
- `internal/ui/` — 共通 UI コンポーネント

## Development workflow

- ビルドは WASM (`GOOS=js GOARCH=wasm go build ./...`) と通常ビルド両方
  が通ること
- テストは `go test ./...` で全部通ること
- testdata は TOML ファイルで宣言的に書く（Go コードを書かずにテスト
  ケースが追加できる）

### コミット直前の必須チェック

**コミットする直前**に、必ず以下を実行する:

1. **批判的セルフコードレビュー**: 自分の変更したコードを他人の PR を
   レビューするつもりで読み直す。特に以下を疑う。
   - 死んだ分岐（到達しない `nil` チェック、常に真/偽の条件）
   - 重複・冗長な抽象化
   - 過剰な防御的プログラミング
   - 命名の一貫性、コメントが自明な箇所を繰り返していないか
   - 追加されたがどこからも使われていないフィールド・関数
   - 仕様を満たしていないエッジケース

2. **`/simplify` スキルの実行**: 手動レビューの後に必ず `/simplify` を
   起動し、再利用性・品質・効率の観点で残っている問題を機械的に潰す。

3. **上記 1, 2 で見つかった問題は必ず修正してからコミットする。**
   「一応残しておく」は禁止（`CLAUDE.md` の既存実装ポリシーと同じ）。

このチェックは小さな修正でもスキップしない。PR を開く前の最後の関門。
