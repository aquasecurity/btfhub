setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'
    load 'test_helper/bats-file/load'

    DIR="$( cd "$( dirname "$BATS_TEST_FILENAME" )" >/dev/null 2>&1 && pwd )"
    PATH="$DIR/../tools:$PATH"
}

@test "run btfgen.sh with example bpf object for x86_64" {
    run btfgen.sh -a x86_64 -o test/goldens/example.bpf.o
    assert_success

    # check ubuntu
    diff custom-archive/ubuntu/20.04/x86_64/5.4.0-39-generic.btf test/goldens/custom-archive/x86_64/5.4.0-39-generic.btf
    diff custom-archive/ubuntu/20.04/x86_64/5.8.0-63-generic.btf test/goldens/custom-archive/x86_64/5.8.0-63-generic.btf
    diff custom-archive/ubuntu/20.04/x86_64/5.11.0-1009-aws.btf test/goldens/custom-archive/x86_64/5.11.0-1009-aws.btf
    diff custom-archive/ubuntu/20.04/x86_64/5.8.0-63-generic.btf test/goldens/custom-archive/x86_64/5.8.0-63-generic.btf

    # check fedora
    diff custom-archive/fedora/29/x86_64/5.3.11-100.fc29.x86_64.btf test/goldens/custom-archive/x86_64/5.3.11-100.fc29.x86_64.btf
    diff custom-archive/fedora/30/x86_64/5.6.13-100.fc30.x86_64.btf test/goldens/custom-archive/x86_64/5.6.13-100.fc30.x86_64.btf
    diff custom-archive/fedora/33/x86_64/5.8.15-301.fc33.x86_64.btf test/goldens/custom-archive/x86_64/5.8.15-301.fc34.x86_64.btf
    diff custom-archive/fedora/34/x86_64/5.11.12-300.fc34.x86_64.btf test/goldens/custom-archive/x86_64/5.11.12-300.fc34.x86_64.btf

    # check centos
    diff custom-archive/centos/8/x86_64/4.18.0-147.8.1.el8_1.x86_64.btf test/goldens/custom-archive/x86_64/4.18.0-147.8.1.el8_1.x86_64.btf
    diff custom-archive/centos/8/x86_64/4.18.0-193.28.1.el8_2.x86_64.btf test/goldens/custom-archive/x86_64/4.18.0-193.28.1.el8_2.x86_64.btf
    diff custom-archive/centos/8/x86_64/4.18.0-348.2.1.el8_5.x86_64.btf test/goldens/custom-archive/x86_64/4.18.0-348.2.1.el8_5.x86_64.btf

}

@test "run btfgen.sh with example bpf object for arm64" {
    run btfgen.sh -a arm64 -o test/goldens/example.bpf.o
    assert_success

    # check ubuntu
    diff custom-archive/ubuntu/20.04/arm64/5.4.0-39-generic.btf test/goldens/custom-archive/arm64/5.4.0-39-generic.btf
    diff custom-archive/ubuntu/20.04/arm64/5.8.0-63-generic.btf test/goldens/custom-archive/arm64/5.8.0-63-generic.btf
    diff custom-archive/ubuntu/20.04/arm64/5.11.0-1009-aws.btf test/goldens/custom-archive/arm64/5.11.0-1009-aws.btf
    diff custom-archive/ubuntu/20.04/arm64/5.8.0-63-generic.btf test/goldens/custom-archive/arm64/5.8.0-63-generic.btf

    # check fedora
    diff custom-archive/fedora/29/arm64/5.3.11-100.fc29.aarch64.btf test/goldens/custom-archive/arm64/5.3.11-100.fc29.aarch64.btf
    diff custom-archive/fedora/30/arm64/5.6.13-100.fc30.aarch64.btf test/goldens/custom-archive/arm64/5.6.13-100.fc30.aarch64.btf
    diff custom-archive/fedora/33/arm64/5.8.15-301.fc33.aarch64.btf test/goldens/custom-archive/arm64/5.8.15-301.fc34.aarch64.btf
    diff custom-archive/fedora/34/arm64/5.11.12-300.fc34.aarch64.btf test/goldens/custom-archive/arm64/5.11.12-300.fc34.x86_64.btf

    # check centos
    diff custom-archive/centos/8/arm64/4.18.0-147.8.1.el8_1.aarch64.btf test/goldens/custom-archive/arm64/4.18.0-147.8.1.el8_1.aarch64.btf
    diff custom-archive/centos/8/arm64/4.18.0-193.28.1.el8_2.aarch64.btf test/goldens/custom-archive/arm64/4.18.0-193.28.1.el8_2.aarch64.btf
    diff custom-archive/centos/8/arm64/4.18.0-348.2.1.el8_5.aarch64.btf test/goldens/custom-archive/arm64/4.18.0-348.2.1.el8_5.aarch64.btf
}
