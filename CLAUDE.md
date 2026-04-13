# CLAUDE.md

本ファイルは Claude Code (claude.ai/code) が本リポジトリで作業する際の
ガイドラインを記述する。

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
