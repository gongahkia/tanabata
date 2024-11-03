![](https://img.shields.io/badge/tako_1.0-build-orange)

>[!IMPORTANT]  
> `Tako` is under active development and awaiting deployment.  
>  
> *\~ Gabriel*

# `tako`

<p align='center'>
    <img src="./asset/logo/tako_mascot.png" width=40% height=40%>
    <br>ランチに何を食べたい？
</p>

*Tako* comprises the following - 

1. *Ta*, a [telegram bot](./bot) that helps you decide where to eat.   
2. *Ko*, a bevy of scrapers that extract public eating spaces in malls, colleges and other locations into a convenient [API wrapper](#api).

## Usage

### How to build

Don't. Access the telegram bot [here](https://t.me/tako_bot).

### For developers

#### Local deployment

*Ta* [telegram bot](./bot) can be deployed locally.

#### Scrapers

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

## Contribute

Tako is open-source. Contribution guidelines are found at [`CONTRIBUTING.md`](./admin/CONTRIBUTING.md).