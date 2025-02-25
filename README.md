# go-expiration-check

## Overview
`go-expiration-check` is a command-line tool to check the expiration dates of domain names using RDAP services.

## Installation
1. Install the tool using `go install`:
    ```sh
    go install github.com/rogafe/go-expiration-check@latest
    ```

## Usage
To use the tool, you can provide domain names directly via command-line flags, environment variables, or standard input.

### Command-line Flags
- `--domain, -d`: Specify domain names to check (comma-separated list).
- `--env, -e`: Specify an environment variable containing domain names.
- `--output, -o`: Specify the output format (`json` or `text`). Default is `text`.

### Examples
1. Check a single domain:
    ```sh
    go-expiration-check check --domain example.com
    ```

2. Check multiple domains:
    ```sh
    go-expiration-check check --domain example.com,example.org
    ```

3. Check domains from an environment variable:
    ```sh
    export DOMAINS="example.com,example.org"
    go-expiration-check check --env DOMAINS
    ```

4. Check domains from standard input:
    ```sh
    go-expiration-check check
    Enter domain names (comma-separated):
    example.com,example.org
    ```

5. Output results in JSON format:
    ```sh
    go-expiration-check check --domain example.com --output json
    ```

### GitHub Actions Example
To use this project with GitHub Actions and check a number of domains every morning at 08:00, you can create a new GitHub Actions workflow in a new empty GitHub project. Follow these steps:

1. Create a new GitHub repository.
2. Add the following workflow file to `.github/workflows/check-domains.yml`:

    ```yaml
        name: Check Domain Expirations

        on:
        schedule:
            - cron: '0 8 * * *'

        jobs:
        check-domains:
            runs-on: ubuntu-latest

            steps:
            - name: Checkout repository
                uses: actions/checkout@v2

            - name: Set up Go
                uses: actions/setup-go@v2
                with:
                go-version: '1.16'

            - name: Install Apprise
                run: |
                pip install apprise

            - name: Install go-expiration-check
                run: |
                go install github.com/rogafe/go-expiration-check@latest

            - name: Check domain expirations
                id: check_domains
                run: |
                # Run the expiration check and output JSON to result.json
                go-expiration-check check --domain example.com,example.org --output json > result.json

            - name: Evaluate domain expiration threshold
                id: evaluate
                run: |
                THRESHOLD=30
                # Check if any domain is expiring in THRESHOLD days or less
                if jq -e ".[] | select(.days_to_expire <= \$THRESHOLD)" result.json > /dev/null; then
                    echo "notification_needed=true" >> $GITHUB_OUTPUT
                else
                    echo "notification_needed=false" >> $GITHUB_OUTPUT
                fi

            - name: Create notification message template
                id: create_message
                run: |
                # Build a message template with the domain details.
                MESSAGE="Domain Expiration Alert:\n"
                while IFS= read -r row; do
                    domain=$(echo "$row" | jq -r '.domain_name')
                    registrar=$(echo "$row" | jq -r '.registrar')
                    expiry=$(echo "$row" | jq -r '.expiry_date')
                    days=$(echo "$row" | jq -r '.days_to_expire')
                    MESSAGE+="\nDomain: $domain\nRegistrar: $registrar\nExpiry Date: $expiry\nDays to Expire: $days\n"
                done < <(jq -c '.[]' result.json)
                echo "$MESSAGE" > message.txt
                # Also output the message to an action output variable if needed
                echo "message<<EOF" >> $GITHUB_OUTPUT
                echo "$MESSAGE" >> $GITHUB_OUTPUT
                echo "EOF" >> $GITHUB_OUTPUT

            - name: Send notification if any domain is near renewal
                if: steps.evaluate.outputs.notification_needed == 'true'
                run: |
                apprise -u "${{ secrets.APPRISE_URL }}" -b "$(cat message.txt)"
    ```

## Contributing
1. Clone the repository:
    ```sh
    git clone https://github.com/rogafe/go-expiration-check.git
    cd go-expiration-check
    ```

2. Install dependencies:
    ```sh
    go mod tidy
    ```

## License
This project is licensed under the MIT License.