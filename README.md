# `tako`

<p align='center'>
    <img src="./asset/logo/tako_mascot.png" width=40% height=40%>
    <br>ランチに何を食べたい？
</p>

*Tako* comprises the following - 

1. *Ta*, a web app that helps you decide where to eat.   
2. *Ko*, a bevy of scrapers that extract public eating spaces in malls, colleges and other locations into a convenient [API wrapper](#api).

## Usage

### How to build it

Don't. Access the web app [here](addalinkherelater.com).

### Architecture

```mermaid
flowchart TB
    
    subgraph "Frontend"

        direction TB
        browser([Browser])
        pageLoad[Page load event]

        subgraph "API"
            geoAPI[Google geolocation API]
            placesAPI[Google places API]
        end

        subgraph "Scrapers"
            bs4[BeautifulSoup]
            pw[Playwright]
        end

    end

    subgraph "Backend"

        subgraph "Django"
            direction TB
            views[[Django views]]
            drf[[Django REST framework]]
            serializers{{DRF serializers}}
            models{{Django models}}
            signals{{Django signals}}
        end

        subgraph "Database"
            direction TB
            postgres[(PostgreSQL)]
        end
    
    end
    
    browser --> pageLoad
    pageLoad --->|triggers| geoAPI & placesAPI & bs4 & pw
    geoAPI & placesAPI & bs4 & pw -->|provisions| views
    views --> drf
    drf -->|validate| serializers
    serializers -->|write data| models
    models -->|CRUD| postgres
    models -->|emit| signals
    postgres -->|query| models
    models -->|fetch| serializers
    serializers -->|serialize| drf
    drf -->|provisions| browser
    user((User)) -.access.-> browser
    drf -.access.-> dev((Developer))
```

### API

*Ko* [scrapers](./scrapers) extract shop data to an array of json following the below structure.

```json
{
  "name": string, // establishment name
  "location": string, // establishment address
  "description": string, // detailed information (operating hours, dietary restrictions etc.)
  "category": string, // identifying category
  "url": string // web url
}
```

### Internal reference

For testing purposes.

```console
$ make config
$ make
```

## Contribute

Tako is open-source. Contribution guidelines are found at [`CONTRIBUTING.md`](./admin/CONTRIBUTING.md).