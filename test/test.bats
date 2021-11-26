setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'

    DIR="$( cd "$( dirname "$BATS_TEST_FILENAME" )" >/dev/null 2>&1 && pwd )"
    PATH="$DIR/../tools:$PATH"
}


@test "run update.sh with ubuntu/bionic and ensure no errors" {
    run update.sh bionic
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with ubuntu/focal and ensure no errors" {
    run update.sh focal
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/29 and ensure no errors" {
    run update.sh fedora29
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/30 and ensure no errors" {
    run update.sh fedora30
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/31 and ensure no errors" {
    run update.sh fedora31
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/32 and ensure no errors" {
    run update.sh fedora32
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/33 and ensure no errors" {
    run update.sh fedora33
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with fedora/34 and ensure no errors" {
    run update.sh fedora34
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with centos/7 and ensure no errors" {
    run update.sh centos7
    assert_success
    refute_output --partial 'ERROR'
}

@test "run update.sh with centos/8 and ensure no errors" {
    run update.sh centos8
    assert_success
    refute_output --partial 'ERROR'
}
