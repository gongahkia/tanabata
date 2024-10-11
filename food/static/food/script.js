fetch('/api/food-places/')
    .then(response => response.json())
    .then(data => {
        const foodList = document.getElementById('food-list');
        data.forEach(place => {
            const div = document.createElement('div');
            div.innerHTML = `<strong>${place.name}</strong> - ${place.location} - $${place.price}`;
            foodList.appendChild(div);
        });
    });