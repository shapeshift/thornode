#!/usr/bin/python3

# pip3 install gitlab-python
import gitlab
import sys

if len(sys.argv) == 1:
    exit('usage: ./prlog <milestone name>')

gl = gitlab.Gitlab()
tn = gl.projects.get(13422983) # thornode project id

mrs = tn.mergerequests.list(milestone=sys.argv[1], status='Merged', all=True)

for idx, m in enumerate(mrs):
    print(f"{idx+1}) {m.title} PR: {m.web_url}")

