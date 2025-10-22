import sys
import json

def main()->list:
    data=sys.stdin.read()
    words=json.loads(data)
    return words


if __name__=="__main__":
    print(main())