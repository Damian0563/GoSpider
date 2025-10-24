import sys
import json
import nltk
from nltk.stem import WordNetLemmatizer


def main()->list:
    data=sys.stdin.read()
    words=json.loads(data)
    tokenized=nltk.word_tokenize(words)
    tags=nltk.pos_tag(tokenized)
    lemmatizer=WordNetLemmatizer()
    result=[]
    for entry in tags:
        word,word_type=entry[0],entry[1]
        word=word.lower()
        if word_type.startswith("NN"): result.append(lemmatizer.lemmatize(word,"n"))
        elif word_type.startswith("V"): result.append(lemmatizer.lemmatize(word,"v"))
        elif word_type.startswith("JJ"): result.append(lemmatizer.lemmatize(word,"a"))
    return json.dumps(result)

if __name__=="__main__":
    nltk.download('punkt')
    nltk.download('punkt_tab')
    nltk.download('averaged_perceptron_tagger_eng')
    nltk.download('averaged_perceptron_tagger')
    nltk.download('wordnet')
    print(main())