document.addEventListener("DOMContentLoaded", () => {
    const foodList = document.getElementById('food-list');
    
    // If you decide to use the API later, you can uncomment this
    /*
    fetch('/api/food-places/')
        .then(response => response.json())
        .then(data => {
            data.forEach(place => {
                const div = document.createElement('div');
                div.innerHTML = `<strong>${place.name}</strong> - ${place.location} - $${place.price}`;
                foodList.appendChild(div);
            });
        });
    */
});