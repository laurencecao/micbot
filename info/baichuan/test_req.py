import argparse
import requests
import sys

def main():
    parser = argparse.ArgumentParser(description="Generate SOAP note from dialogue and history.")
    parser.add_argument("--dialogue", required=True, help="Patient dialogue text")
    parser.add_argument("--history", required=True, help="Patient medical history")
    parser.add_argument("--url", default="http://localhost:8000/generate_soap", help="API Endpoint")

    args = parser.parse_args()

    payload = {
        "dialogue": open(args.dialogue, 'r').read(),
        "history": open(args.history, 'r').read(),
    }

    try:
        response = requests.post(args.url, json=payload)
        response.raise_for_status()
        print(response.text)
    except requests.exceptions.RequestException as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
