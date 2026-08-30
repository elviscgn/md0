class Md0 < Formula
  desc "Safe computational Markdown runtime"
  homepage "https://github.com/elviscgn/md0"
  head "https://github.com/elviscgn/md0.git", branch: "main"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -buildid="), "./cmd/md0"
  end

  test do
    assert_match "md0 v", shell_output("#{bin}/md0 version")
    (testpath/"report.md").write <<~EOS
      md0: 0.1
      Value: @input value number = 2
      Result: {{ value }}
    EOS
    assert_match "valid md0/PURE", shell_output("#{bin}/md0 validate report.md")
  end
end
