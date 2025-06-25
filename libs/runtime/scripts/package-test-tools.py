"""
Provides package tools for running automated tests.
"""

import os
import argparse
import subprocess
import time
import json
from pathlib import Path
from typing import Dict, cast

import boto3
from boto3_type_annotations.secretsmanager import Client as SecretsManagerClient
from boto3_type_annotations.sqs import Client as SQSClient
from dotenv import dotenv_values

parser = argparse.ArgumentParser(
    description='package test tools for rust-common crates')
parser.add_argument('--localdeps', action='store_true',
                    help='bring up dependencies (LocalStack, MySQL etc.) locally')


class LocalstackNotReady(Exception):
    """
    Custom exception type that is raised
    when Localstack is not ready in time.
    """


class TestRunnerFailed(Exception):
    """
    Custom exception type that is raised
    when the tests or the test runner fail.
    """


def run_package_test_tools() -> None:
    """
    Runs the service test tools, depending on the type of tests requested
    this will prepare the environment differently.
    This will run integration and unit tests.
    """
    args = parser.parse_args()
    try:
        _run_integration_tests(args)
    except (KeyboardInterrupt, LocalstackNotReady):
        if args.localdeps:
            _teardown_local_deps()


def _run_integration_tests(args: argparse.Namespace) -> None:
    """
    Runs unit and integration tests combined.
    """
    if args.localdeps:
        _run_local_deps(context="test")
    env_copy = os.environ.copy()
    in_ci_env = os.environ.get('GITHUB_ACTIONS')
    test_env_file = '.env.test-ci' if in_ci_env else '.env.test'
    report_flags = "--lcov --output-path coverage.lcov" if in_ci_env else "--html"
    completed_process = subprocess.run(
        # By default cargo will run all tests for the workspace
        # including unit and integration tests.
        f'cargo llvm-cov {report_flags} test --workspace -- --color always --nocapture --show-output',
        env={
            # Integration tests can't share the same env vars (.env) as the API running
            # in docker as the host endpoints for the AWS service mocks are different.
            **cast(Dict[str, str], dotenv_values(test_env_file)),
            **env_copy,
        },
        shell=True,
        check=False
    )
    if args.localdeps:
        _teardown_local_deps()
    if completed_process.returncode != 0:
        raise TestRunnerFailed(
            f'Tests or test runner failed with code {completed_process.returncode}'
        )


DEPS_DOCKER_COMPOSE = 'docker-compose.test-deps.yml'
LOCALSTACK_EDGE_ENDPOINT = 'http://localhost:44566'


def _run_local_deps(context: str) -> None:
    """
    Brings up local dependencies needed to run tests.
    """
    print('Tearing down previous instances of local dependencies ...')
    _teardown_local_deps()

    print('Bringing up dependencies (LocalStack etc.) ...')
    subprocess.run(
        f'docker compose -f {DEPS_DOCKER_COMPOSE} up -d',
        shell=True, check=True
    )

    print('Waiting for sevices to be ready, tailing LocalStack logs for "Ready." ...')
    _wait_for_localstack(deadline_seconds=60)

    _populate_secrets(context)
    _generate_sqs_queues(context)


def _teardown_local_deps(print_context: bool = False) -> None:
    if print_context:
        print('Tearing down local dependencies ...')

    subprocess.run(
        f'docker compose -f {DEPS_DOCKER_COMPOSE} stop',
        shell=True, check=True
    )
    subprocess.run(
        f'docker compose -f {DEPS_DOCKER_COMPOSE} rm -v -f',
        shell=True, check=True
    )


def _wait_for_localstack(deadline_seconds: int) -> None:
    start_time = time.time()
    with subprocess.Popen(
        ['docker', 'logs', '-f', 'localstack_celerity_runtime_tests'],
        stdout=subprocess.PIPE
    ) as process:
        if process.stdout is not None:
            for line in process.stdout:
                current_time = time.time()
                if current_time >= start_time + deadline_seconds:
                    message = f'Timed out waiting for LocalStack to be ready after {deadline_seconds} seconds'
                    print(message)
                    process.kill()
                    raise LocalstackNotReady(message)
                line_str = line.decode('utf-8')
                if line_str.strip() == 'Ready.':
                    print('LocalStack services are ready, continuing ...')
                    process.kill()
                    break


def _populate_secrets(context: str) -> None:
    print("Saving secrets to LocalStack Secret Manager ...")
    client: SecretsManagerClient = boto3.client(
        'secretsmanager',
        region_name='eu-west-2',
        endpoint_url=LOCALSTACK_EDGE_ENDPOINT
    )
    secrets_file_path = 'secrets.local.json' if context == 'local' else 'secrets.test.json'
    if Path(secrets_file_path).is_file():
        with open(secrets_file_path, mode='r', encoding='utf8') as handle:
            secret_string = handle.read()
            client.create_secret(
                Name=os.environ.get('RUST_COMMON_SECRET_ID'),
                SecretString=secret_string
            )
    else:
        print("No secrets file, moving on ...")


JSON_EXT = '.json'


def _generate_sqs_queues(context: str) -> None:
    context_folder = 'tests' if context == 'test' else 'local'
    print('Generating SQS Queues in LocalStack SQS ...')
    client: SQSClient = boto3.client(
        'sqs', region_name='eu-west-2',
        endpoint_url=LOCALSTACK_EDGE_ENDPOINT
    )
    queues_path = f'{context_folder}/__data/sqs/queues'
    if not Path(queues_path).is_dir():
        print('No sqs queues fixture directory. moving on ...')

    queue_files = [
        pos_json for pos_json in os.listdir(
            queues_path
        ) if pos_json.endswith(JSON_EXT)
    ]
    for queue_file in queue_files:
        print(f'Creating SQS queue defined in {queue_file}')
        with open(os.path.join(queues_path, queue_file), mode='r', encoding='utf8') as handle:
            queue_definition = json.load(handle)
            output = client.create_queue(
                QueueName=queue_file.replace('.json', ''),
                Attributes=queue_definition
            )
            print(output)


if __name__ == "__main__":
    run_package_test_tools()
