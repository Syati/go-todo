# 環境別の設定

設定は Viper で読み込みます。`grpc_template` と同じ、環境別ファイルに個人用のローカルファイルを重ねる方式です。

## ファイル配置

```text
config/settings/
  development.yml
  development.local.yml  # 任意、Git 管理対象外
  test.yml
  test.local.yml         # 任意、Git 管理対象外
  production.yml
  production.local.yml   # 任意、Git 管理対象外
```

`--config-dir` 未指定時は、外部の `<env>.yml` がなければバイナリ内の標準設定を使います。
`--config-dir` を指定した場合は、そのディレクトリの `<env>.yml` が必須です。
`<env>.local.yml` が存在する場合だけ、内容を重ねます。
共通設定ファイルは不要で、環境ごとのファイルにない値はコードのデフォルト値を使います。
標準の `development`・`test`・`production` だけを明示的に埋め込み、個人用の `.local.yml` は埋め込みません。

## 環境の選択

優先順位は **`--env` > `APP_ENV` > `development`** です。

リポジトリのルートでビルドして実行します。

```sh
rtk make build

# development（デフォルト）
rtk proxy ./bin/go-todo serve

# 環境変数で選択
rtk proxy env APP_ENV=development ./bin/go-todo serve

# フラグが APP_ENV より優先される
rtk proxy env APP_ENV=production ./bin/go-todo --env development serve
```

環境名は英小文字で始まり、英小文字・数字・ハイフン・アンダースコアを使えます。
独自の環境を追加する場合は、対応する `<env>.yml` を用意してください。

## 設定値の優先順位

**環境変数 > `<env>.local.yml` > 外部の `<env>.yml`（なければ埋め込み設定）> コードのデフォルト値** の順です。
外部の環境別ファイルが存在する場合は、それを使います。埋め込み設定とのマージは行いません。

例えば `config/settings/development.local.yml` に次の内容を置くと、個人の開発環境だけポートを変更できます。

```yaml
addr: "127.0.0.1:9090"
```

さらに `ADDR` を指定すると、ローカルファイルより優先されます。

```sh
rtk proxy env ADDR=127.0.0.1:9091 ./bin/go-todo serve
```

空文字の環境変数は未設定として扱います。例えば `ADDR=''` でファイルの値を消すことはできません。
`.env` は自動で読み込みません。

## 設定項目

| YAML | 環境変数 | 内容 |
| --- | --- | --- |
| `database_url` | `DATABASE_URL` | PostgreSQL の接続URL。必須 |
| `addr` | `ADDR` | HTTP の待受先。コードの初期値は `127.0.0.1:8080` |
| `cookie_secure` | `COOKIE_SECURE` | Secure Cookie の有効・無効。コードの初期値は `false` |
| `bootstrap_email` | `BOOTSTRAP_EMAIL` | 初期特権管理者のメールアドレス |
| `bootstrap_password` | `BOOTSTRAP_PASSWORD` | 初期特権管理者のパスワード |

`bootstrap_email` と `bootstrap_password` は、使う場合は両方指定してください。
`admin bootstrap` コマンドが空の DB に初期管理者を作成します。`serve` は管理者を作成しません。

同梱の環境別設定は次のとおりです。

| 環境 | HTTP の待受先 | Secure Cookie | DB 接続先 |
| --- | --- | --- | --- |
| `development` | `127.0.0.1:8080` | 無効 | Compose のローカル DB |
| `test` | `127.0.0.1:8081` | 無効 | `DATABASE_URL` で指定 |
| `production` | `0.0.0.0:8080` | 有効 | `DATABASE_URL` で指定 |

本番の接続情報やパスワードは環境変数から渡してください。
本番環境には開発用の DB 接続先を引き継ぎません。

なお、Go の統合テストで使う `TEST_DATABASE_URL` はテストコード専用の変数です。
`APP_ENV=test` の CLI が読む `DATABASE_URL` とは別で、互いに代用されません。

## 設定ディレクトリの指定

`--config-dir` 未指定時は、実行時のカレントディレクトリから `config/settings` を探します。
環境別ファイルがなければ埋め込み設定を使うので、バイナリ単体を別のディレクトリに置いて実行できます。
別の場所に配置する場合は絶対パスで指定できます。

```sh
rtk proxy ./bin/go-todo --env production --config-dir /etc/go-todo/settings serve
```

この例では `/etc/go-todo/settings/production.yml` と、存在する場合は `production.local.yml` を読みます。
`--env` と `--config-dir` は、`serve`・`db up/down/status`・`admin bootstrap` で共通です。
`db new` とヘルプの表示には、設定の読み込みや DB 接続は不要です。
`db up/down/status` は埋め込み SQL を使い、外部の migration ディレクトリは不要です。
開発用の `db new` の出力先はカレントディレクトリの `db/migrations` で、このフラグでは変更されません。

## 実装方針

- [internal/config/config.go](../internal/config/config.go) の `Load()` が Viper を作成し、ファイルの読み込み・型変換・検証を行います。
- `AutomaticEnv()` で環境変数を参照します。型付き `Config` に変換するとき、ファイルに存在しない設定項目も対象になるよう `BindEnv()` で既知のキーを登録しています。
- 読み込んだ設定は型付きの `Config` として渡します。HTTP ハンドラーが Viper を直接参照する構成にはしていません。
- `init()` では設定を読みません。Cobra がフラグを処理した後、各コマンドから `Load()` を呼びます。
- `runtime.Caller()` でソースの位置を探さず、`--config-dir` で配布先の配置に対応します。
- 標準設定は [config/embed.go](../config/embed.go)、migration は [db/migrations/embed.go](../db/migrations/embed.go) で埋め込みます。変更を配布する場合は再ビルドが必要です。

## エラーと検証

外部ファイルも対応する埋め込み設定もない場合や、`--config-dir` で指定した必須ファイルがない場合はエラーになります。
任意のローカルファイルは、存在しない場合だけ無視します。読み込み権限・YAML の構文などの問題はエラーとして返します。
未知の設定キー、不正な型、PostgreSQL 以外の接続URL、不正な待受先、初期管理者情報の片方だけの指定も検証します。

設定に関するテストは [internal/config/config_test.go](../internal/config/config_test.go) にあります。

```sh
rtk mise exec -- go test -race ./internal/config ./internal/cli
```
