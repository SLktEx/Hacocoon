# GitHub Git capability plugin

このパッケージは `haco git push` が利用する、オプションの GitHub 向け Git capability を実装します。

これは Core ドメインの一部ではなく adapter/plugin です。Core が持つのは汎用の capability 契約と policy 判定だけです。Git remote の解析、repository/branch の権限境界、承認後の変更検知、最終的な brokered `git push` など Git/GitHub 固有の処理はこのパッケージが担当します。

この plugin は Sandbox に GitHub credential を渡しません。GitHub repository、target ref、source commit、force-push の意味を capability service が評価した後にだけ push を実行します。

現時点の Hacocoon の plugin は通常の Go package 境界と静的 composition を使います。dynamic shared-object/plugin loader を導入するものではありません。
