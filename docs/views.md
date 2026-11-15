# 画面の開発

画面は [templ](https://templ.guide/) で記述し、Go コードを生成します。
全画面を1つのテンプレートにまとめず、レイアウト・共通部品・各画面を分けています。

## 配置と責務

```text
internal/
  domain/                  管理者・商品・SKU の型と入力検証
  app/                     HTTP ハンドラー、認証、CSRF Cookie、HTTP 応答
  views/
    layout.templ           HTML 文書、管理画面・認証画面の共通レイアウト
    components/
      sidebar.templ        サイドバーと現在位置
      header.templ         ヘッダーとログアウトフォーム
      forms.templ          CSRF、エラー、ステータス・権限の表示
      pagination.templ     ページ切り替えと検索条件の保持
    auth/login.templ
    dashboard/index.templ
    faq/index.templ
    products/
      index.templ
      show.templ
      form.templ
    skus/form.templ
    admins/
      index.templ
      show.templ
      form.templ
```

`*_templ.go` は同じディレクトリに生成されます。
編集対象は `.templ` で、生成物も一緒に Git で管理します。
templ のバージョンは `go.mod` で固定しています。グローバルに別の templ をインストールする必要はありません。

## レイアウトと型付きの引数

[layout.templ](../internal/views/layout.templ) の `Console` は管理画面、`Auth` はログイン画面のレイアウトです。
共通情報は `views.Page` にまとめ、商品などのデータは各画面の引数として渡します。

```go
type Page struct {
    Title string
    Admin domain.Admin
    CSRF  string
}
```

例えば商品のフォームは、共通レイアウトに画面の内容を渡します。

```templ
templ Form(page views.Page, product domain.Product, errorMessage string) {
    @views.Console(page, components.ProductsSection) {
        @components.Alert(errorMessage)
        <form method="post" action={ templ.SafeURL(Path(product.ID)) }>
            @components.CSRF(page.CSRF)
            <input name="name" value={ product.Name } required/>
            <button>保存する</button>
        </form>
    }
}
```

`Console` は `{ children... }` の位置にフォームを配置します。
条件分岐・繰り返しには Go の `if`・`for` を使い、別の部品は `@components.CSRF(...)` のように呼び出します。
引数や参照するフィールドの不整合は、生成された Go コードのビルドで検出されます。

## HTTP ハンドラーからの呼び出し

ハンドラーは必要なデータを取得し、描画するコンポーネントを直接選びます。
画面名の文字列による切り替えは行いません。

```go
component := productviews.Form(
    a.viewData(w, r, "商品入力"),
    product,
    validationError,
)
a.render(w, r, component, http.StatusUnprocessableEntity)
```

`viewData()` がログイン中の管理者と CSRF トークンを準備します。
`render()` は `templ.Component` をバッファに描画してから HTTP ヘッダーと本文を送信します。
描画に失敗した場合は、途中までの HTML を送らずに 500 を返します。

表示に使う型は `internal/domain` に置きます。views から HTTP ハンドラーや DB クエリは呼び出しません。
各一覧の検索条件やページ番号は、その画面の `IndexData` と共通の `components.Pagination` で受け取ります。

## 編集・生成・確認

リポジトリのルートで実行します。

```sh
rtk make format-views
rtk make test
rtk make check
rtk make build
```

| コマンド | 内容 |
| --- | --- |
| `make format-views` | `.templ` の整形と画面の生成 |
| `make generate-views` | templ だけ生成 |
| `make generate-sql` | sqlc だけ生成 |
| `make generate` | sqlc と templ を生成 |
| `make run` / `make test` / `make build` | templ を生成してから各処理を実行 |
| `make check` | 生成物の更新漏れを検査し、`go vet` を実行 |

新しい画面は対応するパッケージに `.templ` を追加し、ハンドラーから型付きの関数を呼び出します。
共通化するのは、レイアウトや複数画面で使う部品です。画面固有のフォームや表はその画面のパッケージに置きます。

## 検証範囲

[描画テスト](../internal/views/render_test.go) では、フォームの送信先・CSRF トークン・文字列のエスケープ・検索条件の保持・権限別の表示を確認します。
[HTTP 応答のテスト](../internal/app/render_test.go) では描画失敗時の応答を確認します。
既存の PostgreSQL 統合テストは、templ で返した画面を含めて認証・商品/SKU・管理者の操作を検証します。
DB を使う統合テストの実行方法は [README](../README.md#検証) を参照してください。

これらはブラウザー操作やスクリーンショットによる検証ではありません。
