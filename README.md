# go-todo

`rails8-todo` の管理画面を Go で実装したアプリです。

- Go 1.27.4 / `net/http` / templ
- PostgreSQL 18 / pgx の `database/sql` ドライバー
- goose: SQL migration の履歴管理
- sqlc: SQL から型付きのクエリメソッドを生成
- Cobra: CLI、Viper: 設定、Testify: テスト

## 実装済みの機能

- メールアドレス・パスワードによるログイン、ログアウト、12時間のセッション
- Dashboard、レスポンシブなサイドバー、FAQ
- 商品の登録・閲覧・更新・削除、名前・ステータス検索、並び替え、30件単位のページング
- SKU の登録・更新・削除、コードの大文字正規化、コードとバリエーションの重複防止
- 管理者の一覧・詳細・検索、特権管理者による登録・更新・論理削除
- 自己削除・自分の権限変更の禁止、管理者編集時の既存セッション失効

商品ステータスは `draft` / `on_sale` / `discontinued`、税率は 8% / 10% です。
SKU の価格は 0〜10,000,000 円、在庫は 0 以上の整数です。

OAuth / JSON API、メール確認、メールによるパスワード再設定、OmniAuth、多言語化、Rails 側のデプロイ構成は未移植です。

## ローカル起動

mise、Docker Compose が必要です。リポジトリのルートで実行します。

```sh
rtk mise install
rtk make up
rtk make migrate
rtk make seed
rtk make dev
```

<http://127.0.0.1:8081> を開き、開発用管理者 `admin@example.com` / `password123` でログインします。
`make dev` は Air で Go サーバーを再起動し、`.templ` の変更時には templ を再生成します。ビルド成功後はブラウザーも自動更新します。通常起動には `make run` を使います。
seed は `rails8-todo` と同じ開発用管理者・商品/SKU を登録します。再実行しても既存データは変更しません。
データは `db/seeds/development/data/` の `admins.yml`・`products.yml` で管理し、バイナリに埋め込みます。
対象データと追加手順は [seed の使い方](docs/seeds.md) を参照してください。

開発環境以外で初期特権管理者を作成する場合は、`admin bootstrap` を使います。
以下は zsh の例で、パスワードは12〜72バイトです。入力したパスワードは表示されません。

```zsh
export BOOTSTRAP_EMAIL='admin@example.test'
read -rs 'BOOTSTRAP_PASSWORD?初期管理者のパスワード: '
export BOOTSTRAP_PASSWORD
rtk make bootstrap
unset BOOTSTRAP_PASSWORD BOOTSTRAP_EMAIL
```

bootstrap は管理者が存在する DB を変更しません。
パスワードを忘れた場合は、別の特権管理者に管理画面から更新してもらってください。

DB は `127.0.0.1:5433` で待ち受けます。Compose の認証情報はローカル開発専用です。
`rtk make down` はコンテナを停止します。DB の volume は残ります。

## CLI

`cmd/go-todo/main.go` が実行の入口で、Cobra の定義は `internal/cli` に置きます。
`root.go` は共通フラグとコマンドの登録を担当し、各コマンドの定義・処理は
`serve.go`・`db.go`・`seed.go`・`admin.go` に分けています。
設定の読み込みは各コマンドの実行時に行うため、ヘルプ表示には設定ファイルや DB 接続が不要です。

```sh
rtk make build
rtk proxy ./bin/go-todo --help
rtk proxy ./bin/go-todo serve
rtk proxy ./bin/go-todo db up
rtk proxy ./bin/go-todo db status
rtk proxy ./bin/go-todo db new add_product_fields
rtk proxy ./bin/go-todo db seed
rtk proxy ./bin/go-todo admin bootstrap
rtk proxy ./bin/go-todo --env production serve
rtk proxy ./bin/go-todo --env test --config-dir /path/to/config db status
```

`db down` / `rtk make rollback` は直前の migration を1件戻します。
初期 migration の rollback はアプリの全テーブルとデータを削除します。
サーバー起動は migration を実行しません。起動前に `db up` を実行してください。
`db up/down/status` はバイナリに埋め込んだ migration を使います。
開発用の `db new` はカレントディレクトリの `db/migrations` に SQL ファイルを作成します。

## バイナリ単体での配布

`rtk make build` で生成した `bin/go-todo` だけを配布できます。
画面は templ の生成コードとしてコンパイルし、CSS・JS・標準設定・migration SQL は `go:embed` で埋め込んでいます。
PostgreSQL は別途用意し、本番では `DATABASE_URL` を環境変数に設定してください。

```sh
# 配布先で実行（DATABASE_URL を設定した状態）
rtk proxy ./go-todo --env production db up
rtk proxy ./go-todo --env production serve
```

外部ファイルなしで起動でき、`--config-dir` や環境変数で設定を変更できます。
`.local.yml` は埋め込みません。埋め込み対象を変更した場合は再ビルドしてください。

## 設定（Viper）

読み込み順、ローカル設定の例、環境切り替え、実装方針は [環境別の設定](docs/configuration.md) を参照してください。

`config/settings/<env>.yml` を読み込み、存在する場合は `<env>.local.yml` を重ねます。
設定値の優先順位は **環境変数 > ローカル YAML > 環境別 YAML（なければ埋め込み設定）> コードのデフォルト値** です。
環境の選択は **`--env` > `APP_ENV` > `development`** の順です。

```text
config/settings/
  development.yml         ローカル開発用DB
  development.local.yml   任意の個人設定（Git 管理対象外）
  test.yml                テスト環境（DATABASE_URL 必須）
  production.yml          本番環境（DATABASE_URL 必須、Secure Cookie）
```

`--config-dir` 未指定時は `config/settings` を探し、環境別の `.yml` がなければ埋め込み設定を使います。
`--config-dir` を明示した場合は、そのディレクトリの環境別 `.yml` が必須です。
`.local.yml` は任意です。ファイルの読み込み・構文エラーは報告します。
未知の設定キー・不正な型・存在しない環境はエラーになります。
`.env` は自動で読み込みません。秘密情報は YAML に記録せず、環境変数から渡してください。

| YAML | 環境変数 | 用途・初期値 |
| --- | --- | --- |
| — | `APP_ENV` | 実行環境。初期値 `development` |
| `database_url` | `DATABASE_URL` | 必須。PostgreSQL の接続URL |
| `addr` | `ADDR` | `127.0.0.1:8080` |
| `cookie_secure` | `COOKIE_SECURE` | `false`。HTTPS で運用するときは `true` |
| `bootstrap_email` | `BOOTSTRAP_EMAIL` | 初期特権管理者のメールアドレス |
| `bootstrap_password` | `BOOTSTRAP_PASSWORD` | 初期特権管理者のパスワード |

Cookie は HttpOnly / SameSite=Lax、状態を変更するフォームは CSRF トークンとクロスオリジン保護を使います。
パスワードは bcrypt、セッショントークンは SHA-256 ハッシュで DB に保存します。
ログイン試行はメールアドレスごとに15分間で5回までです。この制限はプロセス内メモリなので、再起動でリセットされ、複数プロセス間では共有されません。

## SQL の変更

```sh
rtk make tools
rtk proxy ./bin/go-todo db new add_product_fields
# db/migrations の SQL の Up / Down を編集
# db/queries の名前付きクエリを編集
rtk make generate
rtk make migrate
```

`sqlc.yaml` は goose の migration をスキーマとして読みます。
生成物 `internal/store` は直接編集せず、SQL の変更と一緒に管理します。
アプリは `store.Queries` と `WithTx` を呼び出し、ハンドラー内に SQL を記述しません。

## 検証

```sh
rtk make test
rtk make lint
rtk make check
rtk make build
```

DB を使わないテストは常に実行されます。PostgreSQL 統合テストには `TEST_DATABASE_URL` が必要です。
未指定時は統合テストが skip されます。ローカル DB で全テストを実行するには:

```sh
rtk proxy env 'TEST_DATABASE_URL=postgres://go_todo:local-development-only@127.0.0.1:5433/go_todo?sslmode=disable' make test
```

統合テストはランダムな専用スキーマに migration を適用し、テスト終了時にそのスキーマを削除します。
テスト用接続ユーザーにはスキーマの作成・削除権限が必要です。
既存のアプリテーブルは変更しません。ブラウザー操作の E2E は含みません。

`rtk make lint` は `golangci-lint run --fix` を実行します。
golangci-lint のバージョンは `mise.toml` で固定し、`.golangci.yml` で標準の linter と gofmt・goimports を有効にしています。
sqlc・templ の生成コードは、自動生成マーカーに基づいて lint・整形の対象から除外します。

GitHub Actions の [CI](.github/workflows/ci.yml) は push・pull request 時に次のジョブを実行します。

- Lint: `make lint` 後に差分を検査し、自動修正や画面生成による変更が残る場合は失敗します。
- Unit test: `make test` で race detector 付きのテストを実行します。DB サービスは起動せず、PostgreSQL 統合テストは skip します。

## 画面の開発（templ）

画面は `internal/views` の `.templ` ファイルを編集します。
共通レイアウト、サイドバーなどの部品、商品・管理者などの画面を分割しています。
構成と追加手順は [画面の開発](docs/views.md) を参照してください。

```sh
rtk make format-views    # .templ を整形し、Go コードを生成
rtk make generate-views  # 画面だけ生成
rtk make generate       # sqlc と templ の両方を生成
```

templ のバージョンは `go.mod` の依存と `tool` 宣言で管理し、`go tool templ` で実行します。
生成物 `*_templ.go` は直接編集せず、`.templ` と一緒に管理します。
`make run`・`make test`・`make build` は画面を自動生成します。
`make check` は生成物の更新漏れを検査してから `go vet` を実行します。

## ディレクトリ

```text
cmd/go-todo/           CLI エントリーポイント
internal/cli/          Cobra のサブコマンド
internal/config/       Viper による設定の読み込み・検証
config/settings/      環境別・ローカルの YAML 設定
internal/app/          認証・HTTP ハンドラー・静的ファイル
internal/domain/       管理者・商品・SKU の型と入力検証
internal/views/        templ のレイアウト・部品・各画面と生成物
internal/store/        sqlc の生成物
db/migrations/        goose migration と埋め込み定義
db/queries/           sqlc の入力SQL
db/seeds/             環境別 seed の入口と開発用データ
```
