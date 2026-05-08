class Sigilbridge < Formula
  desc "Self-hosted AI gateway for model routing, budgets, audit logs, OAuth, CLI agents, and plugins"
  homepage "https://github.com/sigilbridge/sigilbridge"
  version "1.0.0"
  license "Apache-2.0"

  on_macos do
    on_intel do
      url "https://github.com/sigilbridge/sigilbridge/releases/download/v#{version}/sigilbridge_v#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end

    on_arm do
      url "https://github.com/sigilbridge/sigilbridge/releases/download/v#{version}/sigilbridge_v#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/sigilbridge/sigilbridge/releases/download/v#{version}/sigilbridge_v#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end

    on_arm do
      url "https://github.com/sigilbridge/sigilbridge/releases/download/v#{version}/sigilbridge_v#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
  end

  def install
    bin.install Dir["sigilbridge_*/sigilbridge"].first => "sigilbridge"
    pkgshare.install Dir["sigilbridge_*/*.example.yaml"]
  end

  service do
    run [opt_bin/"sigilbridge", "serve", "--config", etc/"sigilbridge/config.yaml"]
    keep_alive true
    log_path var/"log/sigilbridge.log"
    error_log_path var/"log/sigilbridge.err.log"
  end

  test do
    assert_match "sigilbridge", shell_output("#{bin}/sigilbridge version")
  end
end
