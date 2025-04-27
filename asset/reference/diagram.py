from diagrams import Diagram, Cluster
from diagrams.generic.blank import Blank
from diagrams.onprem.ci import GithubActions  
from diagrams.programming.language import Python, Go
from diagrams.generic.storage import Storage
from diagrams.onprem.client import Users
from diagrams.generic.network import Router

graph_attr = {
    "ranksep": "0.7",
    "nodesep": "0.4",  
    "labelloc": "t",
    "labeljust": "c",  
}

with Diagram("Tanabata REST API Architecture", show=False, direction="TB", graph_attr=graph_attr):
    auto_deploy = Blank("Auto-deploy on\nrepo update")
    monthly_scrape = Blank("Monthly scrape")
    with Cluster("GitHub"):
        schedule = GithubActions("Scheduled Action\n(1st of every month)")
        scraper = Python("Scraper Script\n(quotefancy.com)")
        json_file = Storage("quotes.json")
        schedule >> scraper >> json_file
    with Cluster("Render"):
        render = Router("Render Service")
        go_app = Go("Go API Server")
        api = Router("API Endpoints\n/quotes\n/quotes/random\n/quotes/{author}")
        json_file >> render
        render >> go_app >> api
    users = Users("API Consumers")
    api >> users
    auto_deploy >> [json_file, render]
    monthly_scrape >> [schedule, scraper]