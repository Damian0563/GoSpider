import sys
import json

def main()->list:
    data=sys.stdin.read()
    print(data)
    words=json.loads(data)
    return json.dumps(words)


if __name__=="__main__":
    print(main())