{
  description = "Movie Night: P2P streaming environment with Go, MPV(uosc), and WebTorrent";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # 1. 封装一个带有 uosc 和其他常用脚本的 MPV
        # 这样你不需要手动去配置 ~/.config/mpv，环境里自带了
        customMpv = pkgs.mpv.override {
          scripts = [
            pkgs.mpvScripts.uosc           # 现代化的 UI
            pkgs.mpvScripts.mpris          # 媒体控制支持 (Linux)
            pkgs.mpvScripts.thumbfast      # 进度条缩略图
          ];
        };

      in
      {
        devShells.default = pkgs.mkShell {
          # 2. 项目所需的工具链
          buildInputs = with pkgs; [
            # 后端语言
            go
            gopls  # Go 语言服务器 (给 VSCode/Editor 用)
            
            # 核心工具
            customMpv

            # 如果你未来要用 go-mpv (CGO)，需要这些库
            mpv-unwrapped
	    pkg-config
          ];

          # 3. 环境变量设置
          # 告诉 Go 编译器去哪里找 libmpv 的头文件 (为未来 CGO 做准备)
          shellHook = ''
            echo "🎬 Movie Night Dev Environment Loaded!"
            echo "---------------------------------------"
            echo "Tool versions:"
            echo "  Go:         $(go version | awk '{print $3}')"
            echo "  MPV:        $(mpv --version | head -n 1 | awk '{print $2}')"
            echo "  WebTorrent: $(webtorrent --version)"
            echo "---------------------------------------"
            echo "Run 'go run main.go' to start the prototype."
          '';
        };
      }
    );
}
